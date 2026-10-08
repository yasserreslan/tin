// The expected value of the random SHA-2, HMAC, HKDF, SHA-3 and SHAKE cases of tools/ci/crypto_check.tin, from Go's standard library:
// one input line per case ("sha NAME MSG", "hmac NAME KEY MSG", "hkdf NAME IKM SALT INFO SIZE", "sha3 NAME MSG [SIZE]", bytes in hex,
// "-" for empty) on standard input, one line of hex ("-" for empty) per case on standard output.
package main

import (
	"bufio"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha3"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"strconv"
	"strings"
)

func unhex(s string) []byte {
	if s == "-" {
		return nil
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func show(b []byte) string {
	if len(b) == 0 {
		return "-"
	}
	return hex.EncodeToString(b)
}

func newHash(name string) func() hash.Hash {
	switch name {
	case "sha256":
		return sha256.New
	case "sha384":
		return sha512.New384
	case "sha512":
		return sha512.New
	}
	panic("unknown hash " + name)
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<24)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		f := strings.Fields(in.Text())
		switch f[0] {
		case "sha":
			h := newHash(f[1])()
			h.Write(unhex(f[2]))
			fmt.Fprintln(out, show(h.Sum(nil)))
		case "hmac":
			m := hmac.New(newHash(f[1]), unhex(f[2]))
			m.Write(unhex(f[3]))
			fmt.Fprintln(out, show(m.Sum(nil)))
		case "hkdf":
			size, _ := strconv.Atoi(f[5])
			k, err := hkdf.Key(newHash(f[1]), unhex(f[2]), unhex(f[3]), string(unhex(f[4])), size)
			if err != nil {
				panic(err)
			}
			fmt.Fprintln(out, show(k))
		case "sha3":
			msg := unhex(f[2])
			switch f[1] {
			case "sha3_256":
				s := sha3.Sum256(msg)
				fmt.Fprintln(out, show(s[:]))
			case "sha3_512":
				s := sha3.Sum512(msg)
				fmt.Fprintln(out, show(s[:]))
			case "shake128", "shake256":
				size, _ := strconv.Atoi(f[3])
				if f[1] == "shake128" {
					fmt.Fprintln(out, show(sha3.SumSHAKE128(msg, size)))
				} else {
					fmt.Fprintln(out, show(sha3.SumSHAKE256(msg, size)))
				}
			}
		}
	}
}
