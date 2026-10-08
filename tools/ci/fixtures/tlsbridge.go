// A TLS client for checks that speak other protocols over TLS by hand: it listens on $PORT in the clear and, for every connection,
// dials the target over TLS 1.3 offering the given ALPN protocols and copies the bytes both ways, so a check writes and reads the
// protocol itself on a plain socket. The protocol each handshake agreed on is appended to the result file as a line "alpn=NAME" (empty
// when none).
//
//	tlsbridge TARGET_PORT CA_FILE ALPN_LIST RESULT_FILE     ALPN_LIST is comma separated, or "-" to offer none
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
)

func main() {
	target, caFile, alpn, result := os.Args[1], os.Args[2], os.Args[3], os.Args[4]
	pem, err := os.ReadFile(caFile)
	if err != nil {
		panic(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(pem)
	var protos []string
	if alpn != "-" {
		protos = strings.Split(alpn, ",")
	}
	cfg := &tls.Config{RootCAs: pool, ServerName: "localhost", MinVersion: tls.VersionTLS13, NextProtos: protos}
	ln, err := net.Listen("tcp", "127.0.0.1:"+os.Getenv("PORT"))
	if err != nil {
		panic(err)
	}
	var mu sync.Mutex
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go func() {
			defer c.Close()
			t, err := tls.Dial("tcp", "127.0.0.1:"+target, cfg)
			if err != nil {
				mu.Lock()
				f, _ := os.OpenFile(result, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
				fmt.Fprintf(f, "error=%v\n", err)
				f.Close()
				mu.Unlock()
				return
			}
			defer t.Close()
			mu.Lock()
			f, _ := os.OpenFile(result, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			fmt.Fprintf(f, "alpn=%s\n", t.ConnectionState().NegotiatedProtocol)
			f.Close()
			mu.Unlock()
			done := make(chan struct{}, 2)
			go func() { io.Copy(t, c); t.CloseWrite(); done <- struct{}{} }()
			go func() { io.Copy(c, t); c.(*net.TCPConn).CloseWrite(); done <- struct{}{} }()
			<-done
			<-done
		}()
	}
}
