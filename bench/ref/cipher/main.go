// Command cipher reads a corpus on stdin and prints, for each case, what Go's crypto/md5,
// crypto/aes, crypto/des, crypto/rc4 and crypto/cipher (CBC, CTR, CFB, OFB) say.
// tools/ci/fixtures/cipher.tin prints the same lines and tools/ci/cipher_check.tin compares
// them (#755). Hex fields are "-" when empty. A misuse Go panics on (and Tin faults on) prints
// "fault" and its class: keysize, iv, notfull, short or padding.
//
//	M <data>                              md5: Sum, and New written in two halves
//	B <cipher> <key> <data>               every block encrypted, then decrypted back
//	CBC <cipher> <key> <iv> <data>        encryption in two calls split at a block, decryption
//	CTR|CFB|OFB <cipher> <key> <iv> <data> <split>
//	                                      encryption in two calls split at a byte, decryption
//	RC4 <key> <data> <split>              the keystream XOR in two calls split at a byte
//	SHORT <mode> <cipher> <key>           a destination one byte short
//	PAD <bs> <data>                       PKCS #7 padding (the reference below; Go has none)
//	UNPAD <bs> <data>                     PKCS #7 unpadding
//
// <cipher> is aes, des or 3des.
package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/md5"
	"crypto/rc4"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<22), 1<<22)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		f := strings.Fields(in.Text())
		if len(f) == 0 {
			continue
		}
		fmt.Fprintln(out, f[0]+" "+run(f))
	}
}

// run handles one case; a panic (Go's way of refusing a misuse) becomes its fault class.
func run(f []string) (res string) {
	defer func() {
		if r := recover(); r != nil {
			res = "fault " + class(fmt.Sprint(r))
		}
	}()
	switch f[0] {
	case "M":
		d := unhex(f[1])
		h := md5.New()
		h.Write(d[:len(d)/2])
		h.Write(d[len(d)/2:])
		s := md5.Sum(d)
		return hex.EncodeToString(s[:]) + " " + hex.EncodeToString(h.Sum(nil))
	case "B":
		b, err := block(f[1], unhex(f[2]))
		if err != nil {
			return "fault " + class(err.Error())
		}
		d := unhex(f[3])
		bs := b.BlockSize()
		ct := make([]byte, len(d))
		for i := 0; i+bs <= len(d); i += bs {
			b.Encrypt(ct[i:], d[i:])
		}
		pt := make([]byte, len(d))
		for i := 0; i+bs <= len(d); i += bs {
			b.Decrypt(pt[i:], ct[i:])
		}
		return enc(ct) + " " + enc(pt)
	case "CBC":
		b, err := block(f[1], unhex(f[2]))
		if err != nil {
			return "fault " + class(err.Error())
		}
		iv := unhex(f[3])
		d := unhex(f[4])
		e := cipher.NewCBCEncrypter(b, iv)
		ct := make([]byte, len(d))
		half := len(d) / 2 / b.BlockSize() * b.BlockSize()
		e.CryptBlocks(ct[:half], d[:half])
		e.CryptBlocks(ct[half:], d[half:])
		pt := make([]byte, len(ct))
		cipher.NewCBCDecrypter(b, iv).CryptBlocks(pt, ct)
		return enc(ct) + " " + enc(pt)
	case "CTR", "CFB", "OFB":
		b, err := block(f[1], unhex(f[2]))
		if err != nil {
			return "fault " + class(err.Error())
		}
		iv := unhex(f[3])
		d := unhex(f[4])
		split, _ := strconv.Atoi(f[5])
		e, dec := streams(f[0], b, iv)
		ct := make([]byte, len(d))
		e.XORKeyStream(ct[:split], d[:split])
		e.XORKeyStream(ct[split:], d[split:])
		pt := make([]byte, len(ct))
		dec.XORKeyStream(pt, ct)
		return enc(ct) + " " + enc(pt)
	case "RC4":
		c, err := rc4.NewCipher(unhex(f[1]))
		if err != nil {
			return "fault " + class(err.Error())
		}
		d := unhex(f[2])
		split, _ := strconv.Atoi(f[3])
		ct := make([]byte, len(d))
		c.XORKeyStream(ct[:split], d[:split])
		c.XORKeyStream(ct[split:], d[split:])
		return enc(ct)
	case "SHORT":
		b, _ := block(f[2], unhex(f[3]))
		bs := b.BlockSize()
		src := make([]byte, 2*bs)
		dst := make([]byte, 2*bs-1)
		switch f[1] {
		case "cbc":
			cipher.NewCBCEncrypter(b, make([]byte, bs)).CryptBlocks(dst, src)
		case "rc4":
			c, _ := rc4.NewCipher(unhex(f[3]))
			c.XORKeyStream(dst, src)
		default:
			e, _ := streams(strings.ToUpper(f[1]), b, make([]byte, bs))
			e.XORKeyStream(dst, src)
		}
		return "ok"
	case "PAD":
		bs, _ := strconv.Atoi(f[1])
		return enc(pad(unhex(f[2]), bs))
	case "UNPAD":
		bs, _ := strconv.Atoi(f[1])
		d, ok := unpad(unhex(f[2]), bs)
		if !ok {
			return "fault padding"
		}
		return enc(d)
	}
	return "unknown"
}

func block(name string, key []byte) (cipher.Block, error) {
	switch name {
	case "aes":
		return aes.NewCipher(key)
	case "des":
		return des.NewCipher(key)
	}
	return des.NewTripleDESCipher(key)
}

// streams is the encrypting and decrypting stream of a mode.
func streams(mode string, b cipher.Block, iv []byte) (cipher.Stream, cipher.Stream) {
	switch mode {
	case "CTR":
		return cipher.NewCTR(b, iv), cipher.NewCTR(b, iv)
	case "CFB":
		return cipher.NewCFBEncrypter(b, iv), cipher.NewCFBDecrypter(b, iv)
	}
	return cipher.NewOFB(b, iv), cipher.NewOFB(b, iv)
}

// pad is PKCS #7 (RFC 5652 section 6.3): n bytes of value n bring the data to whole blocks.
func pad(d []byte, bs int) []byte {
	n := bs - len(d)%bs
	out := append([]byte{}, d...)
	for i := 0; i < n; i++ {
		out = append(out, byte(n))
	}
	return out
}

func unpad(d []byte, bs int) ([]byte, bool) {
	if len(d) == 0 || len(d)%bs != 0 {
		return nil, false
	}
	n := int(d[len(d)-1])
	if n < 1 || n > bs {
		return nil, false
	}
	for _, c := range d[len(d)-n:] {
		if int(c) != n {
			return nil, false
		}
	}
	return d[:len(d)-n], true
}

// class names a refusal by what its message says, in Go's words or Tin's.
func class(msg string) string {
	switch {
	case strings.Contains(msg, "invalid key size"):
		return "keysize"
	case strings.Contains(msg, "IV length"):
		return "iv"
	case strings.Contains(msg, "not full blocks"):
		return "notfull"
	case strings.Contains(msg, "output smaller than input"), strings.Contains(msg, "len(dst) < len(src)"), strings.Contains(msg, "out of range"):
		return "short"
	}
	return "other: " + msg
}

func enc(b []byte) string {
	if len(b) == 0 {
		return "-"
	}
	return hex.EncodeToString(b)
}

func unhex(s string) []byte {
	if s == "-" {
		return []byte{}
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
