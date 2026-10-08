// The servers of tools/ci/replay_check.tin that the recorded code calls: tiny peers that count their connections, so the check can tell
// a live call from a replayed one. A control server on $PORT makes and inspects them:
//
//	GET /new?kind=K     a new peer on a free port; answers its port. K is one of
//	  redis      GET cart:7 answers "2 books", another GET nil, AUTH +OK, anything else an error
//	  http       answers every request "12.50 for <body>"
//	  echo       reads once, answers "echo <what it read>"
//	  ws         a WebSocket server: the handshake, then each text frame echoed, until a close
//	  payments   declines every charge with a 502
//	  seen       reads once, remembers what it read, answers "$2\r\nok\r\n" (a Redis that shows what it was sent)
//	GET /stats?port=P   "conns=N"
//	GET /seen?port=P    what the seen peer read, in hex
package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

type peer struct {
	kind  string
	mu    sync.Mutex
	conns int
	seen  []byte
}

func parseResp(data []byte) ([][]byte, []byte, bool) {
	if len(data) == 0 || data[0] != '*' {
		return nil, data, false
	}
	e := bytes.Index(data, []byte("\r\n"))
	if e < 0 {
		return nil, data, false
	}
	n, _ := strconv.Atoi(string(data[1:e]))
	at := e + 2
	var parts [][]byte
	for i := 0; i < n; i++ {
		e := bytes.Index(data[at:], []byte("\r\n"))
		if e < 0 {
			return nil, data, false
		}
		size, _ := strconv.Atoi(string(data[at+1 : at+e]))
		at += e + 2
		if len(data) < at+size+2 {
			return nil, data, false
		}
		parts = append(parts, data[at:at+size])
		at += size + 2
	}
	return parts, data[at:], true
}

func (p *peer) redis(c net.Conn) {
	var data []byte
	buf := make([]byte, 4096)
	for {
		n, err := c.Read(buf)
		if err != nil || n == 0 {
			return
		}
		data = append(data, buf[:n]...)
		for bytes.Contains(data, []byte("\r\n")) {
			parts, rest, ok := parseResp(data)
			if !ok {
				break
			}
			data = rest
			switch cmd := strings.ToUpper(string(parts[0])); {
			case cmd == "AUTH":
				io.WriteString(c, "+OK\r\n")
			case cmd == "GET" && string(parts[1]) == "cart:7":
				io.WriteString(c, "$7\r\n2 books\r\n")
			case cmd == "GET":
				io.WriteString(c, "$-1\r\n")
			default:
				io.WriteString(c, "-ERR unknown\r\n")
			}
		}
	}
}

func readRequest(c net.Conn) (string, []byte, bool) {
	r := bufio.NewReader(c)
	head := ""
	length := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return "", nil, false
		}
		head += line
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if i := strings.Index(line, ":"); i > 0 && strings.EqualFold(strings.TrimSpace(line[:i]), "content-length") {
			length, _ = strconv.Atoi(strings.TrimSpace(line[i+1:]))
		}
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return "", nil, false
	}
	return head, body, true
}

func (p *peer) http(c net.Conn) {
	_, body, ok := readRequest(c)
	if !ok {
		return
	}
	reply := "12.50 for " + string(body)
	fmt.Fprintf(c, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(reply), reply)
}

func (p *peer) payments(c net.Conn) {
	if _, _, ok := readRequest(c); !ok {
		return
	}
	reply := "payments: card declined"
	fmt.Fprintf(c, "HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(reply), reply)
}

func (p *peer) echo(c net.Conn) {
	buf := make([]byte, 4096)
	n, _ := c.Read(buf)
	c.Write(append([]byte("echo "), buf[:n]...))
}

func (p *peer) seenPeer(c net.Conn) {
	buf := make([]byte, 4096)
	n, _ := c.Read(buf)
	p.mu.Lock()
	p.seen = append(p.seen, buf[:n]...)
	p.mu.Unlock()
	io.WriteString(c, "$2\r\nok\r\n")
}

func (p *peer) ws(c net.Conn) {
	r := bufio.NewReader(c)
	key := ""
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if i := strings.Index(line, ":"); i > 0 && strings.EqualFold(line[:i], "sec-websocket-key") {
			key = strings.TrimSpace(line[i+1:])
		}
	}
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	fmt.Fprintf(c, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", base64.StdEncoding.EncodeToString(sum[:]))
	for {
		var h [2]byte
		if _, err := io.ReadFull(r, h[:]); err != nil {
			return
		}
		op, n := h[0]&15, int(h[1]&127)
		if n == 126 {
			var l [2]byte
			io.ReadFull(r, l[:])
			n = int(binary.BigEndian.Uint16(l[:]))
		}
		var mask [4]byte
		io.ReadFull(r, mask[:])
		data := make([]byte, n)
		io.ReadFull(r, data)
		for i := range data {
			data[i] ^= mask[i%4]
		}
		if op == 8 {
			c.Write([]byte{0x88, 0x02, 0x03, 0xe8})
			return
		}
		reply := append([]byte("echo: "), data...)
		c.Write(append([]byte{0x81, byte(len(reply))}, reply...))
	}
}

func (p *peer) serve(c net.Conn) {
	defer c.Close()
	p.mu.Lock()
	p.conns++
	p.mu.Unlock()
	switch p.kind {
	case "redis":
		p.redis(c)
	case "http":
		p.http(c)
	case "echo":
		p.echo(c)
	case "ws":
		p.ws(c)
	case "payments":
		p.payments(c)
	case "seen":
		p.seenPeer(c)
	}
}

var (
	lock sync.Mutex
	all  = map[string]*peer{}
)

func main() {
	http.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		p := &peer{kind: r.URL.Query().Get("kind")}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		lock.Lock()
		all[port] = p
		lock.Unlock()
		go func() {
			for {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				go p.serve(c)
			}
		}()
		io.WriteString(w, port)
	})
	find := func(w http.ResponseWriter, r *http.Request) *peer {
		lock.Lock()
		defer lock.Unlock()
		p := all[r.URL.Query().Get("port")]
		if p == nil {
			http.Error(w, "no such peer", 404)
		}
		return p
	}
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		if p := find(w, r); p != nil {
			p.mu.Lock()
			defer p.mu.Unlock()
			fmt.Fprintf(w, "conns=%d", p.conns)
		}
	})
	http.HandleFunc("/seen", func(w http.ResponseWriter, r *http.Request) {
		if p := find(w, r); p != nil {
			p.mu.Lock()
			defer p.mu.Unlock()
			io.WriteString(w, hex.EncodeToString(p.seen))
		}
	})
	panic(http.ListenAndServe("127.0.0.1:"+os.Getenv("PORT"), nil))
}
