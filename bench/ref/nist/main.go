// The nist twin, Go side: read the corpus of tools/ci/nist_check.tin on stdin and print one
// line per case, "KEY result", so the check can compare them with tools/ci/fixtures/nist.tin
// (#927). Go's crypto/elliptic and crypto/dsa, and crypto/x509 for certificate keys, are the
// oracle. A Go panic is the "fault" a Tin fault stands for; a DSA case whose parameters no signature
// can verify under is "bad parameters", as ErrDSAParameters.
//
// Lines (numbers are hex, "-" is an empty byte string, a leading "-" is a negative number):
//
//	IS curve x y                      IsOnCurve: true or false
//	ADD curve x1 y1 x2 y2             Add: "x y" or fault
//	DBL curve x y                     Double: "x y" or fault
//	MUL curve x y k                   ScalarMult: "x y" or fault
//	BASE curve k                      ScalarBaseMult: "x y"
//	DSA p q g y hash r s              dsa.Verify: verified, bad parameters or rejected
//	SPKI der hash sig                 x509.ParsePKIXPublicKey, then dsa.Verify: bad key, verified,
//	                                  bad parameters or rejected
//	FIPS                              fips140.Enabled and Enforced
package main

import (
	"bufio"
	"crypto/dsa"
	"crypto/elliptic"
	"crypto/fips140"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
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

// run handles one case.
func run(f []string) string {
	switch f[0] {
	case "IS":
		c := curve(f[1])
		return fmt.Sprint(c.IsOnCurve(num(f[2]), num(f[3])))
	case "ADD":
		c := curve(f[1])
		return points(func() (*big.Int, *big.Int) {
			return c.Add(num(f[2]), num(f[3]), num(f[4]), num(f[5]))
		})
	case "DBL":
		c := curve(f[1])
		return points(func() (*big.Int, *big.Int) {
			return c.Double(num(f[2]), num(f[3]))
		})
	case "MUL":
		c := curve(f[1])
		return points(func() (*big.Int, *big.Int) {
			return c.ScalarMult(num(f[2]), num(f[3]), bytes(f[4]))
		})
	case "BASE":
		c := curve(f[1])
		return points(func() (*big.Int, *big.Int) {
			return c.ScalarBaseMult(bytes(f[2]))
		})
	case "DSA":
		pub := &dsa.PublicKey{
			Parameters: dsa.Parameters{P: num(f[1]), Q: num(f[2]), G: num(f[3])},
			Y:          num(f[4]),
		}
		return verdict(pub, bytes(f[5]), num(f[6]), num(f[7]))
	case "SPKI":
		key, err := x509.ParsePKIXPublicKey(bytes(f[1]))
		if err != nil {
			return "bad key"
		}
		pub, ok := key.(*dsa.PublicKey)
		if !ok {
			return "bad key"
		}
		var sig struct{ R, S *big.Int }
		rest, err := asn1.Unmarshal(bytes(f[3]), &sig)
		if err != nil || len(rest) != 0 {
			return "rejected"
		}
		return verdict(pub, bytes(f[2]), sig.R, sig.S)
	case "FIPS":
		return fmt.Sprintf("enabled=%v enforced=%v", fips140.Enabled(), fips140.Enforced())
	}
	panic("unknown case " + f[0])
}

// curve is the named NIST curve.
func curve(name string) elliptic.Curve {
	switch name {
	case "P-224":
		return elliptic.P224()
	case "P-256":
		return elliptic.P256()
	case "P-384":
		return elliptic.P384()
	case "P-521":
		return elliptic.P521()
	}
	panic("unknown curve " + name)
}

// points prints the affine point f returns, or "fault" where Go panics.
func points(f func() (*big.Int, *big.Int)) (out string) {
	defer func() {
		if recover() != nil {
			out = "fault"
		}
	}()
	x, y := f()
	return x.Text(16) + " " + y.Text(16)
}

// verdict is dsa.Verify's answer, with the parameter cases that no signature can verify under
// named as bad parameters: P zero, Q at most 1 or not a whole number of bytes long.
func verdict(pub *dsa.PublicKey, hash []byte, r, s *big.Int) (out string) {
	defer func() {
		if recover() != nil {
			out = "panic"
		}
	}()
	p, q := pub.P, pub.Q
	if p.Sign() == 0 || q.Cmp(big.NewInt(1)) <= 0 || q.BitLen()%8 != 0 {
		return "bad parameters"
	}
	if dsa.Verify(pub, hash, r, s) {
		return "verified"
	}
	return "rejected"
}

// num reads a hex number, with an optional leading minus.
func num(s string) *big.Int {
	n, ok := new(big.Int).SetString(s, 16)
	if !ok {
		panic("bad number " + s)
	}
	return n
}

// bytes reads a hex byte string; "-" is empty.
func bytes(s string) []byte {
	if s == "-" {
		return []byte{}
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
