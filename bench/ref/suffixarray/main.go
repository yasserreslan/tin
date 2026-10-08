package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"index/suffixarray"
	"os"
	"sort"
	"strconv"
	"strings"
)

func main() {
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		f := strings.Fields(s.Text())
		if len(f) != 3 { panic("expected text pattern limit") }
		decode := func(v string) []byte { if v == "-" { return nil }; b, err := hex.DecodeString(v); if err != nil { panic(err) }; return b }
		data := decode(f[0])
		pattern := decode(f[1])
		n, err := strconv.Atoi(f[2]); if err != nil { panic(err) }
		x := suffixarray.New(data)
		lookup := x.Lookup(pattern, n)
		all := x.Lookup(pattern, -1)
		sort.Ints(all)
		fmt.Printf("%v|%v\n", lookup, all)
	}
	if err := s.Err(); err != nil { panic(err) }
}
