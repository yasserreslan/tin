// Command gob is the Go side of tools/ci/gob_check.tin: Go's encoding/gob for the corpus of seeds
// that tools/ci/fixtures/gob.tin encodes and decodes (#913).
//
//	encode <seed>     prints the stream of the seed's values as hex (one process per seed: Go's
//	                  type ids are process-wide, so a seed's ids are those of a fresh process)
//	decode            reads "<seed> <hex>" lines on stdin and prints one "R <value>" line per
//	                  value of the stream, or "ERR <message>" for a stream that does not decode
//
// A seed picks a kind of value (seed % 11) and a count of values (1 + seed/11 % 3). The values come
// from a generator the Tin side repeats call for call: a 64-bit linear congruential state with
// wrapping arithmetic, so both sides build the same values.
package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// Inner is a struct nested in Rec and in slices.
type Inner struct {
	A int
	B string
}

// Rec has every kind of field the corpus uses.
type Rec struct {
	Ints []int
	U    uint
	F    float64
	S    string
	Bs   []byte
	Flag bool
	In   Inner
	M    map[string]int
	Arr  []Inner
}

type rng struct{ s uint64 }

func (r *rng) next() uint64 {
	r.s = r.s*6364136223846793005 + 1442695040888963407
	return r.s
}

func (r *rng) int() int64 {
	x := int64(r.next())
	k := r.next() % 64
	v := x >> k
	if r.next()%4 == 0 {
		v = 0
	}
	return v
}

func (r *rng) uint() uint64 {
	u := r.next() >> (r.next() % 64)
	if r.next()%4 == 0 {
		u = 0
	}
	return u
}

func (r *rng) float() float64 {
	if r.next()%4 == 0 {
		return 0
	}
	v := int64(r.next() >> (r.next() % 64))
	return float64(v) / float64(int64(1)<<(r.next()%30))
}

func (r *rng) bytes(max uint64) []byte {
	n := r.next() % (max + 1)
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r.next())
	}
	return b
}

func (r *rng) flag() bool { return r.next()%2 == 0 }

func (r *rng) inner() Inner {
	a := int(r.int())
	return Inner{A: a, B: string(r.bytes(6))}
}

func (r *rng) rec() Rec {
	var x Rec
	n := r.next() % 5
	for i := uint64(0); i < n; i++ {
		x.Ints = append(x.Ints, int(r.int()))
	}
	x.U = uint(r.uint())
	x.F = r.float()
	x.S = string(r.bytes(8))
	x.Bs = r.bytes(8)
	x.Flag = r.flag()
	x.In = r.inner()
	if r.next()%2 == 0 {
		x.M = map[string]int{string(r.bytes(4)): int(r.int())}
	}
	m := r.next() % 4
	for i := uint64(0); i < m; i++ {
		x.Arr = append(x.Arr, r.inner())
	}
	return x
}

// value returns the i-th value of the seed's stream, boxed for gob.
func value(kind int, r *rng) any {
	switch kind {
	case 0:
		return r.inner()
	case 1:
		return r.rec()
	case 2:
		return r.int()
	case 3:
		return string(r.bytes(8))
	case 4:
		var s []int
		n := r.next() % 5
		for i := uint64(0); i < n; i++ {
			s = append(s, int(r.int()))
		}
		return s
	case 5:
		m := map[string]int{}
		if r.next()%2 == 0 {
			m[string(r.bytes(4))] = int(r.int())
		}
		return m
	case 6:
		return r.float()
	case 7:
		return r.flag()
	case 8:
		return uint64(r.uint())
	case 9:
		var s []Inner
		n := r.next() % 4
		for i := uint64(0); i < n; i++ {
			s = append(s, r.inner())
		}
		return s
	}
	return r.bytes(8)
}

func counts(seed int) int { return 1 + (seed/11)%3 }

func encodeSeed(seed int) []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	r := &rng{s: uint64(seed)}
	kind := seed % 11
	for i := 0; i < counts(seed); i++ {
		if err := enc.Encode(value(kind, r)); err != nil {
			fmt.Fprintln(os.Stderr, "encode:", err)
			os.Exit(1)
		}
	}
	return buf.Bytes()
}

// text prints a value of the seed's kind, as the Tin side prints its decoded value.
func text(v any) string {
	switch x := v.(type) {
	case Inner:
		return inner(x)
	case Rec:
		parts := []string{}
		for _, n := range x.Ints {
			parts = append(parts, fmt.Sprint(n))
		}
		arr := []string{}
		for _, a := range x.Arr {
			arr = append(arr, inner(a))
		}
		return fmt.Sprintf("Rec{Ints:[%s] U:%d F:%v S:%s Bs:%s Flag:%t In:%s M:%s Arr:[%s]}",
			strings.Join(parts, " "), x.U, x.F, hex.EncodeToString([]byte(x.S)), hex.EncodeToString(x.Bs), x.Flag,
			inner(x.In), mapText(x.M), strings.Join(arr, " "))
	case map[string]int:
		return mapText(x)
	case []int:
		parts := []string{}
		for _, n := range x {
			parts = append(parts, fmt.Sprint(n))
		}
		return "[" + strings.Join(parts, " ") + "]"
	case []Inner:
		parts := []string{}
		for _, a := range x {
			parts = append(parts, inner(a))
		}
		return "[" + strings.Join(parts, " ") + "]"
	case string:
		return hex.EncodeToString([]byte(x))
	case []byte:
		return hex.EncodeToString(x)
	case float64:
		return fmt.Sprintf("%v", x)
	case bool:
		return fmt.Sprintf("%t", x)
	case uint64:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	}
	return fmt.Sprintf("?%T", v)
}

func inner(x Inner) string {
	return fmt.Sprintf("Inner{A:%d B:%s}", x.A, hex.EncodeToString([]byte(x.B)))
}

func mapText(m map[string]int) string {
	parts := []string{}
	for k, v := range m {
		parts = append(parts, hex.EncodeToString([]byte(k))+":"+fmt.Sprint(v))
	}
	return "map{" + strings.Join(parts, " ") + "}"
}

// decodeSeed reads the values of a stream and prints each one.
func decodeSeed(seed int, stream []byte, out *bufio.Writer) {
	dec := gob.NewDecoder(bytes.NewReader(stream))
	kind := seed % 11
	for i := 0; i < counts(seed); i++ {
		var v any
		switch kind {
		case 0:
			var x Inner
			if err := dec.Decode(&x); err != nil {
				fmt.Fprintf(out, "ERR %s\n", err)
				return
			}
			v = x
		case 1:
			var x Rec
			if err := dec.Decode(&x); err != nil {
				fmt.Fprintf(out, "ERR %s\n", err)
				return
			}
			v = x
		case 2:
			var x int64
			if err := dec.Decode(&x); err != nil {
				fmt.Fprintf(out, "ERR %s\n", err)
				return
			}
			v = x
		case 3:
			var x string
			if err := dec.Decode(&x); err != nil {
				fmt.Fprintf(out, "ERR %s\n", err)
				return
			}
			v = x
		case 4:
			var x []int
			if err := dec.Decode(&x); err != nil {
				fmt.Fprintf(out, "ERR %s\n", err)
				return
			}
			v = x
		case 5:
			var x map[string]int
			if err := dec.Decode(&x); err != nil {
				fmt.Fprintf(out, "ERR %s\n", err)
				return
			}
			v = x
		case 6:
			var x float64
			if err := dec.Decode(&x); err != nil {
				fmt.Fprintf(out, "ERR %s\n", err)
				return
			}
			v = x
		case 7:
			var x bool
			if err := dec.Decode(&x); err != nil {
				fmt.Fprintf(out, "ERR %s\n", err)
				return
			}
			v = x
		case 8:
			var x uint64
			if err := dec.Decode(&x); err != nil {
				fmt.Fprintf(out, "ERR %s\n", err)
				return
			}
			v = x
		case 9:
			var x []Inner
			if err := dec.Decode(&x); err != nil {
				fmt.Fprintf(out, "ERR %s\n", err)
				return
			}
			v = x
		case 10:
			var x []byte
			if err := dec.Decode(&x); err != nil {
				fmt.Fprintf(out, "ERR %s\n", err)
				return
			}
			v = x
		}
		fmt.Fprintf(out, "R %s\n", text(v))
	}
}

func main() {
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	if len(os.Args) >= 3 && os.Args[1] == "encode" {
		var seed int
		fmt.Sscanf(os.Args[2], "%d", &seed)
		fmt.Fprintf(out, "%x\n", encodeSeed(seed))
		return
	}
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	for in.Scan() {
		var seed int
		var h string
		if _, err := fmt.Sscanf(in.Text(), "%d %s", &seed, &h); err != nil {
			fmt.Fprintf(out, "ERR bad line\n")
			continue
		}
		raw, err := hex.DecodeString(h)
		if err != nil {
			fmt.Fprintf(out, "ERR hex\n")
			continue
		}
		decodeSeed(seed, raw, out)
	}
}
