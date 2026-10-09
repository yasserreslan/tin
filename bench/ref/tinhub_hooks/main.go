// Command tinhub_hooks is the receiving end of tinhub's webhook and mail tests (#1023): an HTTP server that checks
// each delivery's X-Tinhub-Signature-256 with Go's crypto/hmac, and an SMTP sink. It prints "http PORT" and
// "smtp PORT", then logs one line per delivery to -out and every mail to -mail.
//
//	/ok     answers 200 to a correctly signed delivery (401 to a badly signed one)
//	/flaky  answers 500 to the first -fail deliveries, then as /ok
//	/down   always answers 500
package main

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

func main() {
	secret := flag.String("secret", "", "the webhooks' secret")
	fails := flag.Int("fail", 2, "how many deliveries /flaky refuses before it accepts")
	out := flag.String("out", "", "the delivery log")
	mail := flag.String("mail", "", "the mail log")
	flag.Parse()
	var mu sync.Mutex
	flaky := 0
	logf := func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		f, err := os.OpenFile(*out, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(f, format+"\n", args...)
		f.Close()
	}
	h := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		m := hmac.New(sha256.New, []byte(*secret))
		m.Write(body)
		want := "sha256=" + hex.EncodeToString(m.Sum(nil))
		ok := hmac.Equal([]byte(want), []byte(r.Header.Get("X-Tinhub-Signature-256")))
		sig := "bad"
		if ok {
			sig = "ok"
		}
		status := 200
		switch {
		case !ok:
			status = 401
		case r.URL.Path == "/down":
			status = 500
		case r.URL.Path == "/flaky":
			mu.Lock()
			flaky++
			if flaky <= *fails {
				status = 500
			}
			mu.Unlock()
		}
		logf("%s %d sig=%s event=%s delivery=%s attempt=%s json=%v", r.URL.Path, status, sig, r.Header.Get("X-Tinhub-Event"), r.Header.Get("X-Tinhub-Delivery"), r.Header.Get("X-Tinhub-Attempt"), strings.HasPrefix(string(body), "{\"kind\":"))
		w.WriteHeader(status)
	}
	hl, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	sl, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	fmt.Printf("http %d\nsmtp %d\n", hl.Addr().(*net.TCPAddr).Port, sl.Addr().(*net.TCPAddr).Port)
	os.Stdout.Sync()
	go func() {
		for {
			c, err := sl.Accept()
			if err != nil {
				return
			}
			go smtpSession(c, func(from string, to []string, data string) {
				mu.Lock()
				defer mu.Unlock()
				f, err := os.OpenFile(*mail, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
				if err != nil {
					panic(err)
				}
				fmt.Fprintf(f, "MAIL FROM=%s TO=%s\n%s\n.\n", from, strings.Join(to, ","), data)
				f.Close()
			})
		}
	}()
	panic(http.Serve(hl, http.HandlerFunc(h)))
}

// smtpSession is a minimal SMTP server: it accepts any sender and recipient and hands each message to deliver.
func smtpSession(c net.Conn, deliver func(string, []string, string)) {
	defer c.Close()
	r := bufio.NewReader(c)
	say := func(s string) { fmt.Fprintf(c, "%s\r\n", s) }
	say("220 tinhub-test ESMTP")
	from := ""
	var to []string
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			say("250 tinhub-test")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			from = strings.Trim(strings.TrimSpace(line)[10:], "<>")
			to = nil
			say("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			to = append(to, strings.Trim(strings.TrimSpace(line)[8:], "<>"))
			say("250 ok")
		case cmd == "DATA":
			say("354 go on")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" || l == ".\n" {
					break
				}
				b.WriteString(strings.TrimPrefix(l, "."))
			}
			deliver(from, to, b.String())
			say("250 queued")
		case cmd == "QUIT":
			say("221 bye")
			return
		default:
			say("250 ok")
		}
	}
}
