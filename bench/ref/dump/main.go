package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
)

var requests = []string{
	"GET /a?q=1 HTTP/1.1\r\nHost: example.test\r\nX-z: one\r\nX-z: two\r\n\r\n",
	"POST /chunk HTTP/1.1\r\nHost: example.test\r\nTransfer-Encoding: chunked\r\nPragma: no-cache\r\n\r\n3\r\nabc\r\n0\r\n\r\n",
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "server" {
		serve(os.Args[2])
		return
	}
	for _, raw := range requests {
		r, err := http.ReadRequest(bufio.NewReader(stringsReader(raw)))
		must(err)
		b, err := httputil.DumpRequest(r, true)
		must(err)
		fmt.Printf("%x\n", b)
	}
	for _, raw := range []string{
		"HTTP/1.1 200 OK\r\nContent-Length: 5\r\nX-z: one\r\nX-z: two\r\n\r\nhello",
		"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n0\r\n\r\n",
		"HTTP/1.1 204 No Content\r\n\r\n",
	} {
		r, err := http.ReadResponse(bufio.NewReader(stringsReader(raw)), nil)
		must(err)
		b, err := httputil.DumpResponse(r, true)
		must(err)
		fmt.Printf("%x\n", b)
	}
}

func serve(addr string) {
	ln, err := net.Listen("tcp", addr)
	must(err)
	for {
		c, err := ln.Accept()
		must(err)
		go func(c net.Conn) {
			defer c.Close()
			r, err := http.ReadRequest(bufio.NewReader(c))
			if err != nil {
				return
			}
			switch r.URL.Path {
			case "/chunked":
				io.WriteString(c, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n0\r\n\r\n")
			case "/nobody":
				io.WriteString(c, "HTTP/1.1 204 No Content\r\n\r\n")
			default:
				io.WriteString(c, "HTTP/1.1 200 OK\r\nContent-Length: 5\r\nX-z: one\r\nX-z: two\r\n\r\nhello")
			}
		}(c)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

type stringReader string

func stringsReader(s string) *stringReader { r := stringReader(s); return &r }
func (r *stringReader) Read(p []byte) (int, error) {
	if len(*r) == 0 {
		return 0, io.EOF
	}
	n := copy(p, string(*r))
	*r = (*r)[n:]
	return n, nil
}
