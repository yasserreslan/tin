// The Go TLS clients of tools/ci/tls_server_check.tin, from crypto/tls and net/http: peers whose behaviour OpenSSL's s_client does not
// give a script. Each subcommand prints what it saw and exits 1 with the reason on a mismatch.
//
//	tlsprobe http ADDR CA FILE       https requests on one keep-alive connection (plain, chunked, streamed and SendFile bodies, a 1 MiB
//	                                 and a chunked upload), ALPN and the version; FILE is the file the server sends for /file
//	tlsprobe pipeline ADDR CA        three pipelined requests answered in order, the end of the stream a close_notify
//	tlsprobe alpn ADDR CA            offers only spdy/3: the server must refuse with no_application_protocol (120)
//	tlsprobe tls12 ADDR CA           a TLS 1.2-only client: prints the version and the answer to GET /fast, or the refusal
//	tlsprobe fragmented ADDR CA      the ClientHello split over many 40-byte records
//	tlsprobe resume ADDR CA          two connections, the second must resume the first's session
//	tlsprobe clientcert ADDR CA CERT KEY   GET /whoami with a client certificate
//	tlsprobe idle ADDR CA N STOPFILE N connections, each after one request, held until STOPFILE exists; prints "ready"
//	tlsprobe hold ADDR N STOPFILE    N TCP connections that never finish a handshake (every fourth stops in its ClientHello), held until
//	                                 STOPFILE exists; prints "ready"
//	tlsprobe slow ADDR CA            sends GET /slow?ms=1500 (keep-alive), prints "sent", then everything up to the end of the stream
//	tlsprobe tamper IN OUT OPENSSL   copies the SSL session in IN to OUT with the last byte of its ticket flipped
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func pattern(n, start int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte((start + i) % 251)
	}
	return b
}

func die(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
	os.Exit(1)
}

func config(ca string) *tls.Config {
	pem, err := os.ReadFile(ca)
	if err != nil {
		die("cannot read %s: %v", ca, err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(pem)
	return &tls.Config{RootCAs: pool, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS13}
}

func get(path string, closeAfter bool) string {
	conn := ""
	if closeAfter {
		conn = "Connection: close\r\n"
	}
	return fmt.Sprintf("GET %s HTTP/1.1\r\nHost: localhost\r\n%s\r\n", path, conn)
}

func httpChecks(addr, ca, file string) {
	cfg := config(ca)
	cfg.NextProtos = []string{"spdy/3", "http/1.1"}
	tr := &http.Transport{TLSClientConfig: cfg, TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}, MaxIdleConnsPerHost: 1}
	c := &http.Client{Transport: tr, Timeout: 60 * time.Second}
	fbytes, err := os.ReadFile(file)
	if err != nil {
		die("cannot read %s: %v", file, err)
	}
	upload := make([]byte, 1<<20)
	rand.Read(upload)
	type check struct {
		method, path string
		body         io.Reader
		chunked      bool
		want         []byte
		prefix       bool
	}
	checks := []check{
		{"GET", "/", nil, false, []byte("hello over tls"), false},
		{"GET", "/info", nil, false, []byte("alpn=http/1.1 suite="), true},
		{"POST", "/echo", bytes.NewReader(upload), false, upload, false},
		{"POST", "/echo", io.NopCloser(strings.NewReader("chunked upload body")), true, []byte("chunked upload body"), false},
		{"GET", "/big?n=3000000", nil, false, pattern(3000000, 0), false},
		{"GET", "/stream?n=40&size=70000", nil, false, pattern(40*70000, 0), false},
		{"GET", "/file", nil, false, fbytes, false},
		{"GET", "/file?n=1000", nil, false, fbytes[:1000], false},
	}
	var proto string
	var version uint16
	for _, k := range checks {
		req, _ := http.NewRequest(k.method, "https://"+addr+k.path, k.body)
		if k.chunked {
			req.ContentLength = -1
		}
		r, err := c.Do(req)
		if err != nil {
			die("%s %s: %v", k.method, k.path, err)
		}
		got, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if r.StatusCode != 200 {
			die("%s %s: status %d %.200s", k.method, k.path, r.StatusCode, got)
		}
		if k.prefix && !bytes.HasPrefix(got, k.want) || !k.prefix && !bytes.Equal(got, k.want) {
			die("%s %s: %d bytes, want %d", k.method, k.path, len(got), len(k.want))
		}
		proto, version = r.TLS.NegotiatedProtocol, r.TLS.Version
	}
	for i := 0; i < 20; i++ {
		r, err := c.Get("https://" + addr + "/fast")
		if err != nil {
			die("/fast: %v", err)
		}
		got, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if string(got) != "fast" {
			die("/fast answered %q", got)
		}
	}
	if proto != "http/1.1" || version != tls.VersionTLS13 {
		die("ALPN %q, version %x", proto, version)
	}
	fmt.Println("ok http")
}

func dial(addr string, cfg *tls.Config) *tls.Conn {
	c, err := tls.DialWithDialer(&net.Dialer{Timeout: 20 * time.Second}, "tcp", addr, cfg)
	if err != nil {
		die("dial: %v", err)
	}
	return c
}

func pipeline(addr, ca string) {
	c := dial(addr, config(ca))
	io.WriteString(c, get("/fast", false)+get("/", false)+get("/info", true))
	data, err := io.ReadAll(c)
	if err != nil {
		die("the stream did not end with close_notify: %v", err)
	}
	var bodies []string
	for _, part := range strings.Split(string(data), "HTTP/1.1 200 OK")[1:] {
		b := strings.SplitN(part, "\r\n\r\n", 2)[1]
		if len(b) > 14 {
			b = b[:14]
		}
		bodies = append(bodies, b)
	}
	if strings.Join(bodies, "|") != "fast|hello over tls|alpn= suite=13" {
		die("pipelined bodies %q", bodies)
	}
	fmt.Println("ok pipeline")
}

func alpn(addr, ca string) {
	cfg := config(ca)
	cfg.NextProtos = []string{"spdy/3"}
	c, err := tls.Dial("tcp", addr, cfg)
	if err == nil {
		c.Close()
		die("handshake succeeded, wanted alert 120")
	}
	if !strings.Contains(err.Error(), "no application protocol") {
		die("wrong refusal: %v", err)
	}
	fmt.Println("alert 120")
}

func tls12(addr, ca string) {
	cfg := config(ca)
	cfg.MinVersion, cfg.MaxVersion = tls.VersionTLS12, tls.VersionTLS12
	c, err := tls.Dial("tcp", addr, cfg)
	if err != nil {
		fmt.Printf("refused %v\n", err)
		return
	}
	io.WriteString(c, get("/fast", true))
	data, _ := io.ReadAll(c)
	fmt.Printf("version %x %s\n", c.ConnectionState().Version, data[bytes.LastIndex(data, []byte("\r\n\r\n"))+4:])
}

// fragConn splits the first record written (the ClientHello) into records of 40 bytes.
type fragConn struct {
	net.Conn
	done bool
}

func (f *fragConn) Write(b []byte) (int, error) {
	if f.done || len(b) < 5 || b[0] != 22 {
		return f.Conn.Write(b)
	}
	f.done = true
	body := b[5:]
	var out []byte
	for i := 0; i < len(body); i += 40 {
		end := i + 40
		if end > len(body) {
			end = len(body)
		}
		hdr := []byte{22, 3, 1, 0, 0}
		binary.BigEndian.PutUint16(hdr[3:], uint16(end-i))
		out = append(append(out, hdr...), body[i:end]...)
	}
	if _, err := f.Conn.Write(out); err != nil {
		return 0, err
	}
	return len(b), nil
}

func fragmented(addr, ca string) {
	raw, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		die("dial: %v", err)
	}
	c := tls.Client(&fragConn{Conn: raw}, config(ca))
	if err := c.Handshake(); err != nil {
		die("handshake: %v", err)
	}
	io.WriteString(c, get("/fast", true))
	data, _ := io.ReadAll(c)
	if !bytes.Contains(data, []byte("fast")) {
		die("answer %q", data)
	}
	fmt.Println("ok fragmented")
}

func resume(addr, ca string) {
	cfg := config(ca)
	cfg.ClientSessionCache = tls.NewLRUClientSessionCache(4)
	var flags []string
	for i := 0; i < 2; i++ {
		c := dial(addr, cfg)
		io.WriteString(c, get("/fast", true))
		data, err := io.ReadAll(c)
		if err != nil || !bytes.HasSuffix(data, []byte("fast")) {
			die("answer %q %v", data, err)
		}
		flags = append(flags, fmt.Sprint(c.ConnectionState().DidResume))
		c.Close()
	}
	fmt.Println("resumed " + strings.Join(flags, " "))
}

func clientcert(addr, ca, certFile, keyFile string) {
	cfg := config(ca)
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		die("%v", err)
	}
	cfg.Certificates = []tls.Certificate{cert}
	c := dial(addr, cfg)
	io.WriteString(c, get("/whoami", true))
	data, _ := io.ReadAll(c)
	fmt.Printf("%s\n", data[bytes.LastIndex(data, []byte("\r\n\r\n"))+4:])
}

func idle(addr, ca string, n int, stop string) {
	cfg := config(ca)
	var conns []*tls.Conn
	buf := make([]byte, 4096)
	for i := 0; i < n; i++ {
		c := dial(addr, cfg)
		io.WriteString(c, get("/fast", false))
		c.Read(buf)
		conns = append(conns, c)
	}
	fmt.Println("ready")
	for {
		if _, err := os.Stat(stop); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	for _, c := range conns {
		c.Close()
	}
}

func hold(addr string, n int, stop string) {
	var conns []net.Conn
	for i := 0; i < n; i++ {
		c, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			die("dial %d: %v", i, err)
		}
		if i%4 == 0 {
			c.Write([]byte("\x16\x03\x01\x01\x00\x01\x00\x00\xfc\x03\x03"))
		}
		conns = append(conns, c)
	}
	fmt.Println("ready")
	for {
		if _, err := os.Stat(stop); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	for _, c := range conns {
		c.Close()
	}
}

func slow(addr, ca string) {
	c := dial(addr, config(ca))
	io.WriteString(c, get("/slow?ms=1500", false))
	fmt.Println("sent")
	data, err := io.ReadAll(c)
	if err != nil {
		fmt.Printf("error %v\n", err)
		return
	}
	fmt.Printf("eof\n%s", data)
}

func tamper(in, out, openssl string) {
	text, err := exec.Command(openssl, "sess_id", "-in", in, "-noout", "-text").Output()
	if err != nil {
		die("sess_id: %v", err)
	}
	lines := strings.Split(string(text), "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "TLS session ticket:" {
			start = i
			break
		}
	}
	if start < 0 {
		die("no ticket in the session")
	}
	re := regexp.MustCompile(`^\s*[0-9a-f]{4} - ((?:[0-9a-f]{2}[ -]?)+)`)
	hexRe := regexp.MustCompile(`[0-9a-f]{2}`)
	var ticket []byte
	for _, l := range lines[start+1:] {
		m := re.FindStringSubmatch(l)
		if m == nil {
			break
		}
		for _, h := range hexRe.FindAllString(m[1], -1) {
			var v byte
			fmt.Sscanf(h, "%02x", &v)
			ticket = append(ticket, v)
		}
	}
	pem, _ := os.ReadFile(in)
	var body strings.Builder
	for _, l := range strings.Split(string(pem), "\n") {
		if !strings.HasPrefix(l, "-----") {
			body.WriteString(strings.TrimSpace(l))
		}
	}
	der, err := base64.StdEncoding.DecodeString(body.String())
	if err != nil {
		die("base64: %v", err)
	}
	at := bytes.Index(der, ticket)
	if len(ticket) <= 32 || at < 0 {
		die("ticket %d bytes at %d", len(ticket), at)
	}
	der[at+len(ticket)-1] ^= 1
	b64 := base64.StdEncoding.EncodeToString(der)
	var sb strings.Builder
	sb.WriteString("-----BEGIN SSL SESSION PARAMETERS-----\n")
	for i := 0; i < len(b64); i += 64 {
		end := i + 64
		if end > len(b64) {
			end = len(b64)
		}
		sb.WriteString(b64[i:end] + "\n")
	}
	sb.WriteString("-----END SSL SESSION PARAMETERS-----\n")
	os.WriteFile(out, []byte(sb.String()), 0o644)
}

func main() {
	a := os.Args
	switch a[1] {
	case "http":
		httpChecks(a[2], a[3], a[4])
	case "pipeline":
		pipeline(a[2], a[3])
	case "alpn":
		alpn(a[2], a[3])
	case "tls12":
		tls12(a[2], a[3])
	case "fragmented":
		fragmented(a[2], a[3])
	case "resume":
		resume(a[2], a[3])
	case "clientcert":
		clientcert(a[2], a[3], a[4], a[5])
	case "idle":
		var n int
		fmt.Sscan(a[4], &n)
		idle(a[2], a[3], n, a[5])
	case "slow":
		slow(a[2], a[3])
	case "hold":
		var n int
		fmt.Sscan(a[3], &n)
		hold(a[2], n, a[4])
	case "tamper":
		tamper(a[2], a[3], a[4])
	default:
		die("unknown subcommand %s", a[1])
	}
}
