// The fake Redis of tools/ci/redis_check.tin: just enough of the protocol (strings, INCR, DEL, PEXPIRE, MULTI/EXEC, AUTH, HELLO,
// SELECT, DEBUG SLEEP SECONDS). It counts reads, so the check can see commands arriving in batches. A control server on $PORT makes
// and inspects servers:
//
//	GET /new?tls=0|1&password=P   a new server on a free port (tls=1: TLS 1.3 with the certificate and key named by the arguments);
//	                              answers its port
//	GET /stats?port=P             "commands=N reads=N"
//	GET /drop?port=P              closes every connection of the server
package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type fake struct {
	password string
	lock     sync.Mutex
	data     map[string]string
	mu       sync.Mutex
	reads    int
	commands int
	conns    []net.Conn
	cfg      *tls.Config
	ln       net.Listener
}

func parse(buf []byte) ([][]byte, []byte, bool) {
	if len(buf) == 0 || buf[0] != '*' {
		return nil, buf, false
	}
	e := bytes.Index(buf, []byte("\r\n"))
	if e < 0 {
		return nil, buf, false
	}
	n, _ := strconv.Atoi(string(buf[1:e]))
	at := e + 2
	var out [][]byte
	for i := 0; i < n; i++ {
		e := bytes.Index(buf[at:], []byte("\r\n"))
		if e < 0 {
			return nil, buf, false
		}
		ln, _ := strconv.Atoi(string(buf[at+1 : at+e]))
		at += e + 2
		if len(buf) < at+ln+2 {
			return nil, buf, false
		}
		out = append(out, buf[at:at+ln])
		at += ln + 2
	}
	return out, buf[at:], true
}

func (f *fake) run(cmd [][]byte) string {
	name := strings.ToUpper(string(cmd[0]))
	f.lock.Lock()
	defer f.lock.Unlock()
	switch name {
	case "PING":
		return "+PONG\r\n"
	case "SELECT", "HELLO":
		return "+OK\r\n"
	case "SET":
		f.data[string(cmd[1])] = string(cmd[2])
		return "+OK\r\n"
	case "GET":
		v, ok := f.data[string(cmd[1])]
		if !ok {
			return "$-1\r\n"
		}
		return fmt.Sprintf("$%d\r\n%s\r\n", len(v), v)
	case "DEL":
		_, ok := f.data[string(cmd[1])]
		delete(f.data, string(cmd[1]))
		if ok {
			return ":1\r\n"
		}
		return ":0\r\n"
	case "INCR":
		cur, _ := strconv.ParseInt(f.data[string(cmd[1])], 10, 64)
		if cur == 1<<63-1 {
			return "-ERR increment or decrement would overflow\r\n"
		}
		cur++
		f.data[string(cmd[1])] = strconv.FormatInt(cur, 10)
		return fmt.Sprintf(":%d\r\n", cur)
	case "PEXPIRE":
		if _, ok := f.data[string(cmd[1])]; ok {
			return ":1\r\n"
		}
		return ":0\r\n"
	}
	return "-ERR unknown command\r\n"
}

func (f *fake) serve(raw net.Conn) {
	var s net.Conn = raw
	if f.cfg != nil {
		t := tls.Server(raw, f.cfg)
		if err := t.Handshake(); err != nil {
			raw.Close()
			return
		}
		s = t
	}
	f.mu.Lock()
	f.conns = append(f.conns, raw)
	f.mu.Unlock()
	defer s.Close()
	var buf []byte
	authed := f.password == ""
	var multi [][][]byte
	inMulti := false
	chunk := make([]byte, 65536)
	for {
		n, err := s.Read(chunk)
		if err != nil || n == 0 {
			return
		}
		f.mu.Lock()
		f.reads++
		f.mu.Unlock()
		buf = append(buf, chunk[:n]...)
		var out strings.Builder
		for {
			cmd, rest, ok := parse(buf)
			if !ok {
				break
			}
			buf = rest
			f.mu.Lock()
			f.commands++
			f.mu.Unlock()
			name := strings.ToUpper(string(cmd[0]))
			if name == "AUTH" {
				if string(cmd[len(cmd)-1]) == f.password {
					authed = true
					out.WriteString("+OK\r\n")
				} else {
					out.WriteString("-WRONGPASS invalid username-password pair\r\n")
				}
				continue
			}
			if !authed {
				out.WriteString("-NOAUTH Authentication required.\r\n")
				continue
			}
			switch {
			case name == "MULTI":
				multi = nil
				inMulti = true
				out.WriteString("+OK\r\n")
				continue
			case name == "EXEC":
				fmt.Fprintf(&out, "*%d\r\n", len(multi))
				for _, c := range multi {
					out.WriteString(f.run(c))
				}
				multi = nil
				inMulti = false
				continue
			case inMulti:
				multi = append(multi, cmd)
				out.WriteString("+QUEUED\r\n")
				continue
			case name == "DEBUG":
				io.WriteString(s, out.String())
				out.Reset()
				secs, _ := strconv.ParseFloat(string(cmd[2]), 64)
				time.Sleep(time.Duration(secs * float64(time.Second)))
				out.WriteString("+OK\r\n")
				continue
			}
			out.WriteString(f.run(cmd))
		}
		if out.Len() > 0 {
			io.WriteString(s, out.String())
		}
	}
}

var (
	lock sync.Mutex
	all  = map[string]*fake{}
)

func main() {
	var cfg *tls.Config
	if len(os.Args) > 2 {
		cert, err := tls.LoadX509KeyPair(os.Args[1], os.Args[2])
		if err != nil {
			panic(err)
		}
		cfg = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}
	}
	http.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		f := &fake{password: r.URL.Query().Get("password"), data: map[string]string{}}
		if r.URL.Query().Get("tls") == "1" {
			f.cfg = cfg
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		f.ln = ln
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		lock.Lock()
		all[port] = f
		lock.Unlock()
		go func() {
			for {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				go f.serve(c)
			}
		}()
		io.WriteString(w, port)
	})
	find := func(w http.ResponseWriter, r *http.Request) *fake {
		lock.Lock()
		defer lock.Unlock()
		f := all[r.URL.Query().Get("port")]
		if f == nil {
			http.Error(w, "no such server", 404)
		}
		return f
	}
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		if f := find(w, r); f != nil {
			f.mu.Lock()
			defer f.mu.Unlock()
			fmt.Fprintf(w, "commands=%d reads=%d", f.commands, f.reads)
		}
	})
	http.HandleFunc("/drop", func(w http.ResponseWriter, r *http.Request) {
		if f := find(w, r); f != nil {
			f.mu.Lock()
			for _, c := range f.conns {
				c.Close()
			}
			f.conns = nil
			f.mu.Unlock()
			io.WriteString(w, "ok")
		}
	})
	panic(http.ListenAndServe("127.0.0.1:"+os.Getenv("PORT"), nil))
}
