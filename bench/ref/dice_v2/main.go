package main

import (
	"fmt"
	"math/rand/v2"
)

// checkRand prints every draw of r in a fixed order, as tools/ci/fixtures/dice_v2.tin does.
func checkRand(label string, r *rand.Rand) {
	ns := []int64{1, 2, 3, 5, 6, 7, 8, 100, 1000, 65536, 1000000007, 4294967295, 4294967296, 4294967297, 4611686018427387907}
	n32 := []int32{1, 2, 3, 7, 100, 1000, 65536, 1000000007, 2147483647}
	xs := make([]int64, 20)
	for i := range xs {
		xs[i] = int64(i)
	}
	for i := 0; i < 300; i++ {
		n := ns[i%len(ns)]
		v1 := r.Int64N(n)
		fmt.Printf("%s %d I64N %d %d\n", label, i, n, v1)
		m := n32[i%len(n32)]
		v2 := r.Int32N(m)
		fmt.Printf("%s %d I32N %d %d\n", label, i, m, v2)
		v3 := r.Int64()
		fmt.Printf("%s %d I64 %d\n", label, i, v3)
		v4 := r.Int()
		fmt.Printf("%s %d INT %d\n", label, i, v4)
		v5 := r.Uint()
		fmt.Printf("%s %d UINT %d\n", label, i, v5)
		f := r.Float64()
		fmt.Printf("%s %d F64 %.17g\n", label, i, f)
		nf := r.NormFloat64()
		fmt.Printf("%s %d NORM %.17g\n", label, i, nf)
		ef := r.ExpFloat64()
		fmt.Printf("%s %d EXP %.17g\n", label, i, ef)
		p := r.Perm(i % 13)
		fmt.Printf("%s %d PERM %v\n", label, i, p)
		r.Shuffle(len(xs), func(a, b int) { xs[a], xs[b] = xs[b], xs[a] })
		fmt.Printf("%s %d SHUF %v\n", label, i, xs)
	}
	// The ziggurat tails (about 1 draw in 1700 for NORM, 1 in 2300 for EXP) call Log.
	for i := 0; i < 20000; i++ {
		tn := r.NormFloat64()
		fmt.Printf("%s %d TNORM %.17g\n", label, i, tn)
		te := r.ExpFloat64()
		fmt.Printf("%s %d TEXP %.17g\n", label, i, te)
	}
}

func main() {
	for seed := uint64(0); seed < 64; seed++ {
		p := rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)
		for i := 0; i < 8; i++ {
			fmt.Printf("P %d %d %d\n", seed, i, p.Uint64())
		}
	}
	for s := 0; s < 8; s++ {
		var seed [32]byte
		for i := range seed {
			seed[i] = byte(s*37 + i*11)
		}
		c := rand.NewChaCha8(seed)
		for i := 0; i < 400; i++ {
			fmt.Printf("C %d %d %d\n", s, i, c.Uint64())
		}
	}
	checkRand("PCG", rand.New(rand.NewPCG(1, 2)))
	checkRand("CHACHA8", rand.New(rand.NewChaCha8([32]byte{})))
}
