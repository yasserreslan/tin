// The TLS servers of tools/ci/tls_check.tin, from Go's crypto/tls: peers that misbehave or answer in ways OpenSSL's s_server does not.
//
//	tls_peer MODE CERT KEY LOG     serves on 127.0.0.1:$PORT; MODE is one of
//	  truncate   TLS 1.3 with ALPN http/1.1: accepts one connection, writes "partial data", then drops the TCP connection under the TLS
//	             layer (no close_notify)
//	  http       net/http over TLS 1.3 (ALPN http/1.1): GET /x answers 15*4096 bytes of "hello over tls ", POST echoes the body chunked;
//	             each request's ALPN is appended to LOG
//	  wss        a WebSocket echo server over TLS 1.3 (ALPN http/1.1); the connection's ALPN is appended to LOG
//	  slow       every connection waits 0.5 s before its handshake, then answers "slow"
//	  tickets    TLS 1.3 that forgets its ticket keys after three connections; "reused=true|false" per connection is appended to LOG
//	  tls12      TLS 1.2 only: answers "HTTP/1.0 200 OK" and the version name
package main

import (
	"bufio"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	logPath string
	logLock sync.Mutex
)

func logLine(s string) {
	logLock.Lock()
	defer logLock.Unlock()
	f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	fmt.Fprintln(f, s)
	f.Close()
}

func main() {
	mode, certFile, keyFile := os.Args[1], os.Args[2], os.Args[3]
	logPath = os.Args[4]
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		panic(err)
	}
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13, NextProtos: []string{"http/1.1"}}
	addr := "127.0.0.1:" + os.Getenv("PORT")
	switch mode {
	case "http":
		srv := &http.Server{Addr: addr, TLSConfig: cfg, TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){}}
		srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" {
				data, _ := io.ReadAll(r.Body)
				w.Header().Set("Transfer-Encoding", "chunked")
				w.Write(data)
				return
			}
			logLine("alpn=" + r.TLS.NegotiatedProtocol)
			body := strings.Repeat("hello over tls ", 4096)
			w.Header().Set("Content-Length", fmt.Sprint(len(body)))
			io.WriteString(w, body)
		})
		// The handshake of the verify run is refused by the client; its error is not news.
		srv.ErrorLog = log.New(io.Discard, "", 0)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			panic(err)
		}
		panic(srv.ServeTLS(ln, "", ""))
	case "tls12":
		cfg.MinVersion, cfg.MaxVersion = tls.VersionTLS12, tls.VersionTLS12
		cfg.NextProtos = nil
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	var keysA, keysB = cfg.Clone(), cfg.Clone()
	var hellos int
	var hellosLock sync.Mutex
	if mode == "tickets" {
		keysA.SessionTicketKey = [32]byte{1}
		keysB.SessionTicketKey = [32]byte{2}
		// The keys change after three ClientHellos (a connection that only connects, like a readiness probe, sends none).
		cfg.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
			hellosLock.Lock()
			defer hellosLock.Unlock()
			hellos++
			if hellos > 3 {
				return keysB, nil
			}
			return keysA, nil
		}
	}
	for {
		raw, err := ln.Accept()
		if err != nil {
			return
		}
		go func() {
			switch mode {
			case "truncate":
				t := tls.Server(raw, cfg)
				if t.Handshake() != nil {
					raw.Close()
					return
				}
				t.Write([]byte("partial data"))
				time.Sleep(200 * time.Millisecond)
				raw.Close() // under the TLS layer: no close_notify
				os.Exit(0)
			case "slow":
				time.Sleep(500 * time.Millisecond)
				t := tls.Server(raw, cfg)
				if t.Handshake() != nil {
					raw.Close()
					return
				}
				buf := make([]byte, 4096)
				t.Read(buf)
				io.WriteString(t, "HTTP/1.1 200 OK\r\nContent-Length: 4\r\nConnection: close\r\n\r\nslow")
				t.Close()
			case "tickets":
				t := tls.Server(raw, cfg)
				if t.Handshake() != nil {
					raw.Close()
					return
				}
				logLine(fmt.Sprintf("reused=%v", t.ConnectionState().DidResume))
				buf := make([]byte, 100)
				t.Read(buf)
				io.WriteString(t, "HTTP/1.0 200 OK\r\n\r\nok")
				t.Close()
			case "tls12":
				t := tls.Server(raw, cfg)
				if t.Handshake() != nil {
					raw.Close()
					return
				}
				buf := make([]byte, 4096)
				t.Read(buf)
				io.WriteString(t, "HTTP/1.0 200 OK\r\n\r\nTLSv1.2")
				t.Close()
			case "wss":
				wss(raw, cfg)
			}
		}()
	}
}

func wss(raw net.Conn, cfg *tls.Config) {
	s := tls.Server(raw, cfg)
	if s.Handshake() != nil {
		raw.Close()
		return
	}
	logLine("alpn=" + s.ConnectionState().NegotiatedProtocol)
	r := bufio.NewReader(s)
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
	fmt.Fprintf(s, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", base64.StdEncoding.EncodeToString(sum[:]))
	for {
		var h [2]byte
		if _, err := io.ReadFull(r, h[:]); err != nil {
			return
		}
		n := int(h[1] & 127)
		if n == 126 {
			var l [2]byte
			io.ReadFull(r, l[:])
			n = int(l[0])<<8 | int(l[1])
		}
		var mask [4]byte
		io.ReadFull(r, mask[:])
		data := make([]byte, n)
		io.ReadFull(r, data)
		for i := range data {
			data[i] ^= mask[i%4]
		}
		if h[0]&15 == 8 {
			s.Write(append([]byte{0x88, byte(len(data))}, data...))
			s.Close()
			return
		}
		out := append([]byte("echo: "), data...)
		s.Write(append([]byte{0x80 | (h[0] & 15), byte(len(out))}, out...))
	}
}
