package main

import (
	"fmt"
	"math/rand/v2"
	"strconv"
)

func main() {
	for seed := uint64(0); seed < 64; seed++ {
		p := rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)
		for i := 0; i < 8; i++ { fmt.Printf("P %d %d %d\n", seed, i, p.Uint64()) }
	}
	var seed [32]byte
	for i := range seed { seed[i] = byte(i) }
	c := rand.NewChaCha8(seed)
	for i := 0; i < 16; i++ { fmt.Printf("C %d %d\n", i, c.Uint64()) }
	r := rand.New(rand.NewPCG(1, 2))
	fmt.Printf("I %d %d\n", r.Int32N(100), r.Int64N(1000000007))
	fmt.Println("F", strconv.FormatFloat(r.Float64(), 'g', 17, 64))
	for i, x := range r.Perm(12) { fmt.Printf("M %d %d\n", i, x) }
}
