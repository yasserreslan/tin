// Command utf16 reads a hex corpus on stdin and prints, for each case, what Go's unicode/utf16 says.
// tools/ci/fixtures/utf16.tin prints the same lines and tools/ci/utf16_check.tin compares them (#753).
//
//	S <hex utf8>    the UTF-16 code units of the string's runes
//	D <hex u16>     the string those code units decode to
//	R <hex runes>   per rune: RuneLen, IsSurrogate and EncodeRune
//	A <hex runes>   AppendRune over the runes
package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"unicode/utf16"
)

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		fields := strings.Fields(in.Text())
		if len(fields) == 0 {
			continue
		}
		arg := func(i int) string {
			if i < len(fields) {
				return fields[i]
			}
			return ""
		}
		switch fields[0] {
		case "S":
			u := utf16.Encode([]rune(string(decodeHex(arg(1)))))
			fmt.Fprintf(out, "S %s\n", hex16(u))
		case "D":
			s := string(utf16.Decode(units(decodeHex(arg(1)))))
			fmt.Fprintf(out, "D %s\n", hex.EncodeToString([]byte(s)))
		case "R":
			fmt.Fprint(out, "R")
			for _, r := range runes(decodeHex(arg(1))) {
				r1, r2 := utf16.EncodeRune(r)
				fmt.Fprintf(out, " %08x:%d:%t:%08x-%08x", uint32(r), utf16.RuneLen(r), utf16.IsSurrogate(r), uint32(r1), uint32(r2))
			}
			fmt.Fprintln(out)
		case "A":
			var a []uint16
			for _, r := range runes(decodeHex(arg(1))) {
				a = utf16.AppendRune(a, r)
			}
			fmt.Fprintf(out, "A %s\n", hex16(a))
		}
	}
}

// hex16 is the four-digit hex of each code unit, run together.
func hex16(u []uint16) string {
	var b strings.Builder
	for _, v := range u {
		fmt.Fprintf(&b, "%04x", v)
	}
	return b.String()
}

// units reads four-hex-digit code units from raw.
func units(raw []byte) []uint16 {
	var out []uint16
	for i := 0; i+1 < len(raw); i += 2 {
		out = append(out, uint16(raw[i])<<8|uint16(raw[i+1]))
	}
	return out
}

// runes reads eight-hex-digit runes from raw.
func runes(raw []byte) []rune {
	var out []rune
	for i := 0; i+3 < len(raw); i += 4 {
		out = append(out, rune(uint32(raw[i])<<24|uint32(raw[i+1])<<16|uint32(raw[i+2])<<8|uint32(raw[i+3])))
	}
	return out
}

func decodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
