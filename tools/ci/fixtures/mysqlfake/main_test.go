package main

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

// The fake MySQL server leaves the TLS handshake on the socket after SSLRequest: it reads exactly one packet and no more.

func pair(t *testing.T) (net.Conn, net.Conn) {
	client, server := net.Pipe()
	t.Cleanup(func() { client.Close(); server.Close() })
	client.SetDeadline(time.Now().Add(time.Second))
	server.SetDeadline(time.Now().Add(time.Second))
	return client, server
}

func TestSSLRequestAndClientHelloInOneWrite(t *testing.T) {
	client, server := pair(t)
	request := make([]byte, 32)
	hello := []byte("\x16\x03\x01\x00\x05hello")
	frame := append([]byte{32, 0, 0, 1}, request...)
	go client.Write(append(frame, hello...))
	s := &session{s: server}
	got, err := s.recv()
	if err != nil || !bytes.Equal(got, request) {
		t.Fatalf("recv = %x, %v", got, err)
	}
	if s.seq != 2 {
		t.Fatalf("seq = %d, want 2", s.seq)
	}
	left := make([]byte, len(hello))
	if _, err := io.ReadFull(server, left); err != nil || !bytes.Equal(left, hello) {
		t.Fatalf("what is left in the socket: %q, %v", left, err)
	}
}

func TestConsecutiveMySQLPackets(t *testing.T) {
	client, server := pair(t)
	go client.Write([]byte("\x03\x00\x00\x00one\x03\x00\x00\x01two"))
	s := &session{s: server}
	for i, want := range []string{"one", "two"} {
		got, err := s.recv()
		if err != nil || string(got) != want {
			t.Fatalf("packet %d = %q, %v", i, got, err)
		}
		if s.seq != i+1 {
			t.Fatalf("seq after packet %d = %d, want %d", i, s.seq, i+1)
		}
	}
}
