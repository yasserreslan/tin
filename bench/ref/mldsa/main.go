// Command mldsa is the Go twin of tools/ci/mldsa_check.tin (#917): Go's crypto/mldsa (Go 1.27) answers the
// ML-DSA cases that tools/ci/fixtures/mldsa.tin answers, so the two can be compared line by line. The decompose
// cases compare Decompose by its definition, which needs no crypto/mldsa.
//
// With no argument it prints the golden corpus: one case per line (the fixture's input form), a tab, and Go's
// class for it: "ok", "fault", or "sha256:" and the digest of the bytes Go returned. The corpus is pinned in
// toolchain/tests/data/mldsa/go_corpus.txt. With the argument "classes" it reads case lines on stdin and prints
// Go's class for each (the drift check, and the check of Tin's randomized signatures, which Go verifies).
// Lines Go's public API cannot answer (the expanded key, and verification of mu) print "unsupported".
package main

import (
	"bufio"
	"crypto"
	"crypto/mldsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// muOpts selects the pre-hashed mode of Go's Sign: the message is the representative mu (crypto.MLDSAMu).
type muOpts struct{}

func (muOpts) HashFunc() crypto.Hash {
	return crypto.MLDSAMu
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "classes" {
		classes(os.Stdin, os.Stdout)
		return
	}
	golden(os.Stdout)
}

// paramsOf is the parameter set named by its number.
func paramsOf(set string) mldsa.Parameters {
	switch set {
	case "44":
		return mldsa.MLDSA44()
	case "65":
		return mldsa.MLDSA65()
	}
	return mldsa.MLDSA87()
}

// chain is n bytes of a SHA-256 chain from label: the corpus's messages, contexts and seeds.
func chain(label string, n int) []byte {
	out := make([]byte, 0, n+32)
	h := sha256.Sum256([]byte(label))
	for len(out) < n {
		out = append(out, h[:]...)
		h = sha256.Sum256(h[:])
	}
	return out[:n]
}

// seedOf is the seed of the i-th key of a parameter set.
func seedOf(set string, i int) []byte {
	s := sha256.Sum256([]byte("tin mldsa corpus seed " + set + " " + strconv.Itoa(i)))
	return s[:]
}

// hexOrDash is b in hex, or "-" when it is empty (the fixture's form of an empty field).
func hexOrDash(b []byte) string {
	if len(b) == 0 {
		return "-"
	}
	return hex.EncodeToString(b)
}

// digest is the class of the bytes b: their SHA-256, so the pinned corpus keeps no signature text.
func digest(b []byte) string {
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}

// field decodes a hex field of a case line; "-" is empty.
func field(s string) ([]byte, error) {
	if s == "-" {
		return []byte{}, nil
	}
	return hex.DecodeString(s)
}

// classOf is Go's class for a result: the digest of the bytes, "ok", or "fault".
func classOf(b []byte, err error) string {
	if err != nil {
		return "fault"
	}
	return digest(b)
}

// evaluate is Go's class for one case line of the fixture's input form.
func evaluate(line string) string {
	f := strings.Fields(line)
	if len(f) < 2 {
		return "unsupported"
	}
	p := paramsOf(f[1])
	switch f[0] {
	case "pk":
		seed, err := field(f[2])
		if err != nil {
			return "fault"
		}
		sk, err := mldsa.NewPrivateKey(p, seed)
		if err != nil {
			return "fault"
		}
		return digest(sk.PublicKey().Bytes())
	case "sign":
		seed, e1 := field(f[2])
		msg, e2 := field(f[3])
		ctx, e3 := field(f[4])
		if e1 != nil || e2 != nil || e3 != nil || f[5] != "-" {
			return "unsupported"
		}
		sk, err := mldsa.NewPrivateKey(p, seed)
		if err != nil {
			return "fault"
		}
		sig, err := sk.SignDeterministic(msg, &mldsa.Options{Context: string(ctx)})
		return classOf(sig, err)
	case "mu":
		seed, e1 := field(f[2])
		mu, e2 := field(f[3])
		if e1 != nil || e2 != nil || f[4] != "-" {
			return "unsupported"
		}
		sk, err := mldsa.NewPrivateKey(p, seed)
		if err != nil {
			return "fault"
		}
		sig, err := sk.SignDeterministic(mu, muOpts{})
		return classOf(sig, err)
	case "decompose":
		g2 := int64(261888)
		if f[1] == "44" {
			g2 = 95232
		}
		return "hex:" + decomposeHash(g2)
	case "vfy":
		pk, e1 := field(f[2])
		msg, e2 := field(f[3])
		ctx, e3 := field(f[4])
		sig, e4 := field(f[5])
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
			return "fault"
		}
		pub, err := mldsa.NewPublicKey(p, pk)
		if err != nil {
			return "fault"
		}
		if mldsa.Verify(pub, msg, sig, &mldsa.Options{Context: string(ctx)}) != nil {
			return "fault"
		}
		return "ok"
	}
	return "unsupported"
}

// decomposeSpec is Decompose (FIPS 204 Algorithm 36) by its definition, for r in [0, q): the high part and the low part
// centred modulo 2 gamma2, with the exception at q - 1. It is variable time, which a reference may be.
func decomposeSpec(r, g2 int64) (int64, int64) {
	const q = 8380417
	alpha := 2 * g2
	r0 := r % alpha
	if r0 > g2 {
		r0 -= alpha
	}
	if r-r0 == q-1 {
		return 0, r0 - 1
	}
	return (r - r0) / alpha, r0
}

// decomposeHash is the hash that tools/ci/fixtures/mldsa.tin prints for decompose: a 64-bit multiply by the FNV prime over
// the high part and r0 + 2^23, for every r below q in order, so the two sides compare the whole function.
func decomposeHash(g2 int64) string {
	h := uint64(1469598103934665603)
	for r := int64(0); r < 8380417; r++ {
		r1, r0 := decomposeSpec(r, g2)
		h = (h ^ uint64(r1)) * 1099511628211
		h = (h ^ uint64(r0+8388608)) * 1099511628211
	}
	return fmt.Sprintf("%016x", h)
}

// classes prints Go's class of each case line of r.
func classes(r io.Reader, w io.Writer) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		fmt.Fprintln(w, evaluate(line))
	}
}

// golden prints the corpus: each case line, a tab, and Go's class for it.
func golden(w io.Writer) {
	emit := func(line string) {
		fmt.Fprintf(w, "%s\t%s\n", line, evaluate(line))
	}
	for _, set := range []string{"44", "65", "87"} {
		p := paramsOf(set)
		for i := 0; i < 3; i++ {
			seed := seedOf(set, i)
			emit("pk " + set + " " + hex.EncodeToString(seed))
			sk, err := mldsa.NewPrivateKey(p, seed)
			if err != nil {
				panic(err)
			}
			pk := sk.PublicKey().Bytes()
			// the first key covers every message length around the SHAKE block boundaries; the others two.
			lengths := []int{0, 136}
			if i == 0 {
				lengths = []int{0, 1, 55, 135, 136, 137, 167, 168, 169, 272, 1000}
			}
			for _, n := range lengths {
				msg := chain("msg "+set+" "+strconv.Itoa(i)+" "+strconv.Itoa(n), n)
				emit("sign " + set + " " + hex.EncodeToString(seed) + " " + hexOrDash(msg) + " - -")
				if i == 0 || n == 136 {
					ctx := chain("ctx "+set+" "+strconv.Itoa(n), 255)
					emit("sign " + set + " " + hex.EncodeToString(seed) + " " + hexOrDash(msg) + " " + hexOrDash(ctx) + " -")
					sig, err := sk.SignDeterministic(msg, &mldsa.Options{Context: string(ctx)})
					if err != nil {
						panic(err)
					}
					// verification lines carry the whole key and signature, so only some lengths are kept
					if n == 0 || n == 136 || n == 1000 {
						emit("vfy " + set + " " + hex.EncodeToString(pk) + " " + hexOrDash(msg) + " " + hexOrDash(ctx) + " " + hex.EncodeToString(sig))
					}
					if i == 0 && n == 136 {
						// tampering: a byte of the signature, the message, the context and the key, a truncated and a longer signature
						bad := append([]byte(nil), sig...)
						bad[0] ^= 1
						emit("vfy " + set + " " + hex.EncodeToString(pk) + " " + hexOrDash(msg) + " " + hexOrDash(ctx) + " " + hex.EncodeToString(bad))
						bad = append([]byte(nil), sig...)
						bad[len(bad)/2] ^= 0x80
						emit("vfy " + set + " " + hex.EncodeToString(pk) + " " + hexOrDash(msg) + " " + hexOrDash(ctx) + " " + hex.EncodeToString(bad))
						bad = append([]byte(nil), sig...)
						bad[len(bad)-1] ^= 0x40
						emit("vfy " + set + " " + hex.EncodeToString(pk) + " " + hexOrDash(msg) + " " + hexOrDash(ctx) + " " + hex.EncodeToString(bad))
						emit("vfy " + set + " " + hex.EncodeToString(pk) + " " + hexOrDash(msg) + " " + hex.EncodeToString(ctx[:254]) + " " + hex.EncodeToString(sig))
						emit("vfy " + set + " " + hex.EncodeToString(pk) + " " + hexOrDash(msg) + " " + hexOrDash(ctx) + " " + hex.EncodeToString(sig[:len(sig)-1]))
						emit("vfy " + set + " " + hex.EncodeToString(pk) + " " + hexOrDash(msg) + " " + hexOrDash(ctx) + " " + hex.EncodeToString(append(append([]byte(nil), sig...), 0)))
						badPk := append([]byte(nil), pk...)
						badPk[0] ^= 1
						emit("vfy " + set + " " + hex.EncodeToString(badPk) + " " + hexOrDash(msg) + " " + hexOrDash(ctx) + " " + hex.EncodeToString(sig))
					}
				}
			}
			if i == 0 {
				// a context of 256 bytes is refused by Sign and Verify (the longest context is 255)
				long := chain("ctx-long "+set, 256)
				emit("sign " + set + " " + hex.EncodeToString(seed) + " - " + hex.EncodeToString(long) + " -")
				// the pre-hashed mode: a signature of a 64-byte mu, and a mu of the wrong length
				mu := chain("mu "+set, 64)
				emit("mu " + set + " " + hex.EncodeToString(seed) + " " + hex.EncodeToString(mu) + " -")
				emit("mu " + set + " " + hex.EncodeToString(seed) + " " + hex.EncodeToString(mu[:63]) + " -")
			}
		}
		// a seed of the wrong size, and a public key of another parameter set
		emit("pk " + set + " " + hex.EncodeToString(seedOf(set, 9)[:31]))
		emit("sign " + set + " " + hex.EncodeToString(append(seedOf(set, 9), 0)) + " - - -")
	}
	// Decompose of every r below q, by the definition, for the two gamma2 values (the rounding of Tin's shifts)
	emit("decompose 44")
	emit("decompose 65")
	// a randomized signature (Go's Sign, a fresh nonce) and its public key: the verification is pinned, not the bytes
	for _, set := range []string{"44", "65", "87"} {
		p := paramsOf(set)
		sk, err := mldsa.GenerateKey(p)
		if err != nil {
			panic(err)
		}
		msg := chain("random "+set, 77)
		ctx := chain("random ctx "+set, 9)
		sig, err := sk.Sign(nil, msg, &mldsa.Options{Context: string(ctx)})
		if err != nil {
			panic(err)
		}
		pk := sk.PublicKey().Bytes()
		emit("vfy " + set + " " + hex.EncodeToString(pk) + " " + hex.EncodeToString(msg) + " " + hex.EncodeToString(ctx) + " " + hex.EncodeToString(sig))
	}
}
