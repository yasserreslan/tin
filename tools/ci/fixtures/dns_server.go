// The fake DNS server of tools/ci/dns_check.tin and dns_cache_check.tin (UDP and TCP, on 127.0.0.1 and 127.0.0.2, port $PORT).
// Every question it receives is appended to the file named by the first argument as "HOST NAME TYPE PROTO" (PROTO udp or tcp), so a
// check counts and orders what the resolver asked. Names:
//
//	ok.test, canonical.test, short.first.test, dots.name, many.dots.name.first.test, rotate.test, fallback.second.test, ttl1.test
//	              answer 127.0.0.1 (A; ttl1.test with a one-second TTL, the others 60)
//	retry.test    dropped by 127.0.0.1, answered by 127.0.0.2
//	trunc.test    truncated over UDP, answered over TCP (the TCP answer is sent in three pieces)
//	wrongid.test  a packet with the wrong ID first, then the right one
//	bad.test      an owner name that points at itself
//	alias.test, inline.test   CNAMEs to canonical.test
//	v6.test       ::1 for AAAA, nothing for A
//	drop.*        never answered
//	anything else NXDOMAIN
package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	logFile *os.File
	logLock sync.Mutex
)

func question(data []byte) (string, uint16, int) {
	if len(data) < 12 {
		panic("short query")
	}
	pos := 12
	var labels []string
	for data[pos] != 0 {
		n := int(data[pos])
		labels = append(labels, string(data[pos+1:pos+1+n]))
		pos += n + 1
	}
	return strings.ToLower(strings.Join(labels, ".")), binary.BigEndian.Uint16(data[pos+1:]), pos + 5
}

func nameBytes(name string) []byte {
	var out []byte
	for _, p := range strings.Split(name, ".") {
		out = append(out, byte(len(p)))
		out = append(out, p...)
	}
	return append(out, 0)
}

func record(owner []byte, kind uint16, data []byte, ttl uint32) []byte {
	out := append([]byte{}, owner...)
	var tail [10]byte
	binary.BigEndian.PutUint16(tail[0:], kind)
	binary.BigEndian.PutUint16(tail[2:], 1)
	binary.BigEndian.PutUint32(tail[4:], ttl)
	binary.BigEndian.PutUint16(tail[8:], uint16(len(data)))
	out = append(out, tail[:]...)
	return append(out, data...)
}

var success = map[string]bool{"ok.test": true, "canonical.test": true, "trunc.test": true, "short.first.test": true,
	"dots.name": true, "many.dots.name.first.test": true, "rotate.test": true, "retry.test": true, "fallback.second.test": true,
	"wrongid.test": true, "ttl1.test": true}

func answer(data []byte, host string, tcp bool) []byte {
	name, typ, end := question(data)
	proto := "udp"
	if tcp {
		proto = "tcp"
	}
	logLock.Lock()
	fmt.Fprintf(logFile, "%s %s %d %s\n", host, name, typ, proto)
	logLock.Unlock()
	if strings.HasPrefix(name, "drop.") || (name == "retry.test" && strings.HasSuffix(host, ".1")) {
		return nil
	}
	flags := uint16(0x8180)
	var answers [][]byte
	switch {
	case name == "trunc.test" && !tcp:
		flags |= 0x200
	case name == "bad.test":
		// A compressed owner referring to itself must be rejected, not hang.
		answers = [][]byte{record([]byte{0xc0 | byte(end>>8), byte(end)}, 1, []byte{127, 0, 0, 1}, 60)}
	case name == "alias.test":
		answers = [][]byte{record([]byte{0xc0, 0x0c}, 5, nameBytes("canonical.test"), 60)}
	case name == "inline.test":
		answers = [][]byte{record(nameBytes("canonical.test"), 1, []byte{127, 0, 0, 1}, 60), record([]byte{0xc0, 0x0c}, 5, nameBytes("canonical.test"), 60)}
	case name == "v6.test":
		if typ == 28 {
			answers = [][]byte{record([]byte{0xc0, 0x0c}, 28, net.ParseIP("::1").To16(), 60)}
		}
	case success[name]:
		if typ == 1 {
			ttl := uint32(60)
			if name == "ttl1.test" {
				ttl = 1
			}
			answers = [][]byte{record([]byte{0xc0, 0x0c}, 1, []byte{127, 0, 0, 1}, ttl)}
		}
	default:
		flags = 0x8183
	}
	out := append([]byte{}, data[:2]...)
	var head [10]byte
	binary.BigEndian.PutUint16(head[0:], flags)
	binary.BigEndian.PutUint16(head[2:], 1)
	binary.BigEndian.PutUint16(head[4:], uint16(len(answers)))
	out = append(out, head[:]...)
	out = append(out, data[12:end]...)
	for _, a := range answers {
		out = append(out, a...)
	}
	return out
}

func serveUDP(c *net.UDPConn, host string) {
	buf := make([]byte, 65535)
	for {
		n, peer, err := c.ReadFromUDP(buf)
		if err != nil {
			return
		}
		data := append([]byte{}, buf[:n]...)
		a := answer(data, host, false)
		if a == nil {
			continue
		}
		if name, _, _ := question(data); name == "wrongid.test" {
			wrong := append([]byte{}, a...)
			wrong[1] ^= 1
			c.WriteToUDP(wrong, peer)
		}
		c.WriteToUDP(a, peer)
	}
}

func serveTCP(l net.Listener, host string) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go func() {
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(3 * time.Second))
			var size [2]byte
			if _, err := readFull(conn, size[:]); err != nil {
				return
			}
			q := make([]byte, binary.BigEndian.Uint16(size[:]))
			if _, err := readFull(conn, q); err != nil {
				return
			}
			a := answer(q, host, true)
			if a == nil {
				return
			}
			frame := append([]byte{byte(len(a) >> 8), byte(len(a))}, a...)
			// Split the length and response body so exact reads must handle fragments.
			for _, part := range [][]byte{frame[:1], frame[1:5], frame[5:]} {
				conn.Write(part)
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}
}

func readFull(c net.Conn, b []byte) (int, error) {
	got := 0
	for got < len(b) {
		n, err := c.Read(b[got:])
		got += n
		if err != nil {
			return got, err
		}
	}
	return got, nil
}

func main() {
	var err error
	logFile, err = os.OpenFile(os.Args[1], os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		panic(err)
	}
	port := os.Getenv("PORT")
	for _, host := range []string{"127.0.0.1", "127.0.0.2"} {
		addr := host + ":" + port
		ua, _ := net.ResolveUDPAddr("udp", addr)
		u, err := net.ListenUDP("udp", ua)
		if err != nil {
			panic(err)
		}
		l, err := net.Listen("tcp", addr)
		if err != nil {
			panic(err)
		}
		go serveUDP(u, host)
		go serveTCP(l, host)
	}
	select {}
}
