package main

// Go twin of toolchain/tests/v2/seal_hasher.tin's 64 MiB stream.

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
)

func main() {
	h := sha256.New()
	h1 := sha1.New()
	chunk := make([]byte, 65536)
	for i := 0; i < 1024; i++ {
		for j := range chunk {
			chunk[j] = byte((i*7 + j*13) & 255)
		}
		h.Write(chunk)
		h1.Write(chunk)
	}
	fmt.Printf("64 MiB sha256: %x\n", h.Sum(nil))
	fmt.Printf("64 MiB sha1: %x\n", h1.Sum(nil))
}
