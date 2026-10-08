// The upstreams of tools/ci/wire_pool_check.tin: small HTTP/1.1 servers that count the connections they accept and the requests on each.
// A control server on $PORT makes and inspects them:
//
//	GET /new?tls=0|1       a new upstream on a free port; answers its port (tls=1 serves TLS 1.3 with the certificate and key named by
//	                       the two arguments)
//	GET /stats?port=P      "accepted=N open=N handshakes=N requests=A,B,C" since the last reset (requests per connection, sorted)
//	GET /reset?port=P      zeroes the counts (open stays)
//
// An upstream answers by path: /ok (also HEAD), /close (Connection: close), /eof (no length, ends with the connection), /old (HTTP/1.0),
// /chunked, /nobody (204), /slow (250 ms), /flaky (the second request on a connection finds it closed), /echo; anything else is a 404.
package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type upstream struct {
	mu         sync.Mutex
	accepted   int
	open       int
	total      int
	handshakes int
	counts     map[int]int
	ln         net.Listener
	cfg        *tls.Config
}

func (u *upstream) accept() {
	for {
		c, err := u.ln.Accept()
		if err != nil {
			return
		}
		go u.serve(c)
	}
}

func (u *upstream) serve(raw net.Conn) {
	u.mu.Lock()
	u.accepted++
	u.open++
	u.total++
	idx := u.total
	u.mu.Unlock()
	conn := raw
	defer func() {
		u.mu.Lock()
		u.open--
		u.mu.Unlock()
		conn.Close()
	}()
	if u.cfg != nil {
		t := tls.Server(raw, u.cfg)
		if err := t.Handshake(); err != nil {
			return
		}
		conn = t
		u.mu.Lock()
		u.handshakes++
		u.mu.Unlock()
	}
	r := bufio.NewReader(conn)
	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			return
		}
		method, path := f[0], f[1]
		n := 0
		for {
			h, err := r.ReadString('\n')
			if err != nil {
				return
			}
			h = strings.TrimRight(h, "\r\n")
			if h == "" {
				break
			}
			if i := strings.Index(h, ":"); i > 0 && strings.EqualFold(h[:i], "content-length") {
				n, _ = strconv.Atoi(strings.TrimSpace(h[i+1:]))
			}
		}
		if n > 0 {
			io.CopyN(io.Discard, r, int64(n))
		}
		u.mu.Lock()
		u.counts[idx]++
		nth := u.counts[idx]
		u.mu.Unlock()
		if !respond(conn, method, path, nth) {
			return
		}
	}
}

// respond answers one request; false when the connection ends with it.
func respond(c net.Conn, method, path string, nth int) bool {
	body := "ok"
	head := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Length: %d\r\n", len(body))
	switch {
	case strings.HasPrefix(path, "/ok"):
		if method == "HEAD" {
			body = ""
		}
		io.WriteString(c, head+"\r\n"+body)
	case strings.HasPrefix(path, "/close"):
		io.WriteString(c, head+"Connection: close\r\n\r\n"+body)
		return false
	case strings.HasPrefix(path, "/eof"):
		io.WriteString(c, "HTTP/1.1 200 OK\r\n\r\nuntil the end")
		return false
	case strings.HasPrefix(path, "/old"):
		io.WriteString(c, "HTTP/1.0 200 OK\r\nContent-Length: 3\r\n\r\nold")
		return false
	case strings.HasPrefix(path, "/chunked"):
		io.WriteString(c, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n3\r\nabc\r\n2\r\nde\r\n0\r\n\r\n")
	case strings.HasPrefix(path, "/nobody"):
		io.WriteString(c, "HTTP/1.1 204 No Content\r\n\r\n")
	case strings.HasPrefix(path, "/slow"):
		time.Sleep(250 * time.Millisecond)
		io.WriteString(c, head+"\r\n"+body)
	case strings.HasPrefix(path, "/flaky"):
		// the second request on a connection finds it closed, as after an idle timeout
		if nth >= 2 {
			return false
		}
		io.WriteString(c, head+"\r\n"+body)
	case strings.HasPrefix(path, "/echo"):
		io.WriteString(c, head+"\r\n"+body)
	default:
		io.WriteString(c, "HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\n\r\n")
	}
	return true
}

var (
	lock sync.Mutex
	all  = map[string]*upstream{}
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
		u := &upstream{counts: map[int]int{}}
		if r.URL.Query().Get("tls") == "1" {
			u.cfg = cfg
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		u.ln = ln
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		lock.Lock()
		all[port] = u
		lock.Unlock()
		go u.accept()
		io.WriteString(w, port)
	})
	find := func(w http.ResponseWriter, r *http.Request) *upstream {
		lock.Lock()
		defer lock.Unlock()
		u := all[r.URL.Query().Get("port")]
		if u == nil {
			http.Error(w, "no such upstream", 404)
		}
		return u
	}
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		u := find(w, r)
		if u == nil {
			return
		}
		u.mu.Lock()
		defer u.mu.Unlock()
		var reqs []int
		for _, n := range u.counts {
			reqs = append(reqs, n)
		}
		sort.Ints(reqs)
		var parts []string
		for _, n := range reqs {
			parts = append(parts, strconv.Itoa(n))
		}
		fmt.Fprintf(w, "accepted=%d open=%d handshakes=%d requests=%s", u.accepted, u.open, u.handshakes, strings.Join(parts, ","))
	})
	http.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		u := find(w, r)
		if u == nil {
			return
		}
		u.mu.Lock()
		u.accepted = 0
		u.counts = map[int]int{}
		u.handshakes = 0
		u.mu.Unlock()
		io.WriteString(w, "ok")
	})
	panic(http.ListenAndServe("127.0.0.1:"+os.Getenv("PORT"), nil))
}
