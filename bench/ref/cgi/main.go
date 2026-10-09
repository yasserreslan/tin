// Command cgi is the Go twin of tools/ci/fixtures/cgi_twin.tin: the same routes, served by
// net/http/cgi (no argument, the CGI variables from the environment) or by net/http/fcgi on a
// loopback port (argument fcgi, which prints the port first).
package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cgi"
	"net/http/fcgi"
	"os"
	"strconv"
)

func routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status/{code}", status)
	mux.HandleFunc("GET /headers", headers)
	mux.HandleFunc("GET /big", big)
	mux.HandleFunc("/", echo)
	return mux
}

// echo answers every other path with the request's line, its headers of interest and its body.
func echo(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "method=%s\npath=%s\nquery=%s\nhost=%s\nctype=%s\nxfoo=%s\nbody-len=%d\nbody=%s\n",
		r.Method, r.URL.Path, r.URL.RawQuery, r.Host, r.Header.Get("Content-Type"), r.Header.Get("X-Foo-Bar"), len(body), body)
}

// status answers with the status code in its path, or 400 when the code is not a number.
func status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	code, err := strconv.Atoi(r.PathValue("code"))
	if err != nil {
		w.WriteHeader(400)
		io.WriteString(w, "bad code\n")
		return
	}
	w.WriteHeader(code)
	fmt.Fprintf(w, "status %d\n", code)
}

// headers sets fields whose names and values the response must spell as Go does.
func headers(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("x-lower", "lower")
	w.Header().Add("X-Space", "  padded  ")
	w.Header().Add("Set-Cookie", "a=1; Path=/")
	w.Header().Add("set-cookie", "b=2")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, "<p>headers</p>\n")
}

// big writes 222000 bytes in 6000 writes.
func big(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for i := 0; i < 6000; i++ {
		io.WriteString(w, "0123456789abcdefghijklmnopqrstuvwxyz\n")
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "fcgi" {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			fail(err)
		}
		fmt.Println(l.Addr().(*net.TCPAddr).Port)
		if err := fcgi.Serve(l, routes()); err != nil {
			fail(err)
		}
		return
	}
	if err := cgi.Serve(routes()); err != nil {
		fail(err)
	}
}
