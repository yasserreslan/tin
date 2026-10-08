// A TCP proxy for tools/ci/tls_server_check.tin that rewrites the first ClientHello's supported_versions from (1.3, 1.2) to (1.2, 1.2),
// as an attacker forcing TLS 1.2 would. It listens on 127.0.0.1:$PORT and forwards to the port in the first argument.
package main

import (
	"bytes"
	"io"
	"net"
	"os"
)

func main() {
	ln, err := net.Listen("tcp", "127.0.0.1:"+os.Getenv("PORT"))
	if err != nil {
		panic(err)
	}
	from := []byte{0x00, 0x2b, 0x00, 0x05, 0x04, 0x03, 0x04, 0x03, 0x03}
	to := []byte{0x00, 0x2b, 0x00, 0x05, 0x04, 0x03, 0x03, 0x03, 0x03}
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go func() {
			u, err := net.Dial("tcp", "127.0.0.1:"+os.Args[1])
			if err != nil {
				c.Close()
				return
			}
			buf := make([]byte, 65536)
			n, _ := c.Read(buf)
			u.Write(bytes.Replace(buf[:n], from, to, 1))
			go func() { io.Copy(c, u); c.Close() }()
			io.Copy(u, c)
			u.Close()
		}()
	}
}
