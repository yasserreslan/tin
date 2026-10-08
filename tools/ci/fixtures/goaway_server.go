// The GOAWAY server of tools/ci/wire_h2_check.tin: cleartext HTTP/2 (prior knowledge) on $PORT that answers "<path> on <connection>".
// It sends GOAWAY after each answer, except for /warm; on a connection where /c came, it waits for a second /c (or one second), answers
// the first and leaves the second out of the GOAWAY (its last stream id is the first's). The number of connections accepted so far is
// written to the file named by the first argument after every accept.
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"sync"
	"time"
)

type frame struct {
	typ, flags byte
	sid        uint32
	payload    []byte
}

func put(c net.Conn, typ, flags byte, sid uint32, payload []byte) {
	h := make([]byte, 9)
	h[0], h[1], h[2] = byte(len(payload)>>16), byte(len(payload)>>8), byte(len(payload))
	h[3], h[4] = typ, flags
	binary.BigEndian.PutUint32(h[5:], sid)
	c.Write(append(h, payload...))
}

func readFrame(c io.Reader) (frame, error) {
	h := make([]byte, 9)
	if _, err := io.ReadFull(c, h); err != nil {
		return frame{}, err
	}
	n := int(h[0])<<16 | int(h[1])<<8 | int(h[2])
	p := make([]byte, n)
	if _, err := io.ReadFull(c, p); err != nil {
		return frame{}, err
	}
	return frame{h[3], h[4], binary.BigEndian.Uint32(h[5:]) & 0x7fffffff, p}, nil
}

// requestPath is the :path of a header block wire wrote: static-table names and plain literals.
func requestPath(block []byte) string {
	i := 0
	integer := func(mask byte) int {
		v := int(block[i] & mask)
		i++
		if v < int(mask) {
			return v
		}
		m := 0
		for {
			b := block[i]
			i++
			v += int(b&127) << m
			m += 7
			if b < 128 {
				return v
			}
		}
	}
	str := func() string {
		if block[i] >= 128 {
			panic("wire sends no Huffman-coded strings")
		}
		n := integer(127)
		s := block[i : i+n]
		i += n
		return string(s)
	}
	names := map[int]string{1: ":authority", 2: ":method", 4: ":path", 6: ":scheme", 58: "user-agent", 28: "content-length", 31: "content-type"}
	for i < len(block) {
		if block[i] >= 128 {
			integer(127)
			continue
		}
		idx := integer(15)
		name := "?"
		if idx == 0 {
			name = str()
		} else if n, ok := names[idx]; ok {
			name = n
		}
		value := str()
		if name == ":path" {
			return value
		}
	}
	return ""
}

func answer(c net.Conn, sid uint32, text string) {
	put(c, 1, 4, sid, []byte{0x88})
	put(c, 0, 1, sid, []byte(text))
}

func goaway(c net.Conn, last uint32) {
	p := make([]byte, 8)
	binary.BigEndian.PutUint32(p, last)
	put(c, 7, 0, 0, p)
}

var (
	mu    sync.Mutex
	count int
)

// serve counts the connection once it has sent the HTTP/2 preface: a probe that only connects is not a connection.
func serve(c net.Conn) {
	defer c.Close()
	preface := make([]byte, 24)
	if _, err := io.ReadFull(c, preface); err != nil || string(preface) != "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n" {
		return
	}
	mu.Lock()
	count++
	n := count
	os.WriteFile(os.Args[1], []byte(fmt.Sprint(count)), 0o644)
	mu.Unlock()
	put(c, 4, 0, 0, nil)
	frames := make(chan frame)
	go func() {
		defer close(frames)
		for {
			f, err := readFrame(c)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					_ = err
				}
				return
			}
			frames <- f
		}
	}()
	var pending []int
	var deadline time.Time
	done := false
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case f, ok := <-frames:
			if !ok {
				return
			}
			if f.typ == 4 && f.flags&1 == 0 {
				put(c, 4, 1, 0, nil)
			}
			if f.typ == 1 && !done {
				path := requestPath(f.payload)
				if path == "/c" {
					pending = append(pending, int(f.sid))
					if deadline.IsZero() {
						deadline = time.Now().Add(time.Second)
					}
				} else {
					answer(c, f.sid, fmt.Sprintf("%s on %d", path, n))
					if path != "/warm" {
						goaway(c, f.sid)
						done = true
					}
				}
			}
		case <-tick.C:
		}
		if len(pending) > 0 && !done && (len(pending) >= 2 || time.Now().After(deadline)) {
			sort.Ints(pending)
			first := uint32(pending[0])
			answer(c, first, fmt.Sprintf("/c on %d", n))
			goaway(c, first)
			done = true
		}
	}
}

func main() {
	ln, err := net.Listen("tcp", "127.0.0.1:"+os.Getenv("PORT"))
	if err != nil {
		panic(err)
	}
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go serve(c)
	}
}
