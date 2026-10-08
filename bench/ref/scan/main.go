// Command scan reads a corpus of source texts on stdin and prints, for each case, every step of Go's text/scanner over it.
// tools/ci/fixtures/scan.tin prints the same lines and tools/ci/scan_check.tin compares them (#753).
//
//	C <mode> <whitespace> <flags> <ops> <hex source>
//
// mode and whitespace are the Scanner's Mode and Whitespace; flags bit 0 sets an identifier predicate ('$' anywhere,
// '-' after the first rune, letters, digits and '_'), bit 1 the file name "src.tin", bit 2 scans another text first and
// calls Init again. ops is repeated until Scan or Next returns EOF, then each of S, N and P runs once more:
//
//	S  Scan: the token, its text in hex, Position (and its offset) and Pos (and its offset)
//	N  Next: the character, Position and Pos
//	P  Peek: the character
//	M  flips ScanStrings, ScanComments and SkipComments in Mode (no output)
//	W  flips '\n' in Whitespace (no output)
//
// Each step is followed by the errors it met (E <position> <offset> <message>), and each case ends with X <ErrorCount>.
package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/scanner"
	"unicode"
)

type failure struct {
	pos scanner.Position
	msg string
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<24), 1<<24)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		f := strings.Fields(in.Text())
		if len(f) < 5 || f[0] != "C" {
			continue
		}
		src := ""
		if len(f) > 5 {
			b, err := hex.DecodeString(f[5])
			if err != nil {
				panic(err)
			}
			src = string(b)
		}
		run(out, num(f[1]), num(f[2]), num(f[3]), f[4], src)
	}
}

func num(s string) uint64 {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		panic(err)
	}
	return n
}

func run(out *bufio.Writer, mode, ws, flags uint64, ops string, src string) {
	fmt.Fprintf(out, "# %d %d %d %s %d\n", mode, ws, flags, ops, len(src))
	var s scanner.Scanner
	var errs []failure
	if flags&2 != 0 {
		s.Filename = "src.tin"
	}
	if flags&1 != 0 {
		s.IsIdentRune = func(ch rune, i int) bool {
			return ch == '$' || ch == '-' && i > 0 || ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch)
		}
	}
	if flags&4 != 0 {
		s.Init(strings.NewReader("first \"text\" 12\n"))
		s.Scan()
		s.Scan()
	}
	s.Init(strings.NewReader(src))
	s.Mode = uint(mode)
	s.Whitespace = ws
	s.Error = func(s *scanner.Scanner, msg string) {
		pos := s.Position
		if !pos.IsValid() {
			pos = s.Pos()
		}
		errs = append(errs, failure{pos, msg})
	}
	flush := func() {
		for _, e := range errs {
			fmt.Fprintf(out, "E %s %d %s\n", e.pos, e.pos.Offset, e.msg)
		}
		errs = errs[:0]
	}
	step := func(op byte) rune {
		r := rune(0)
		switch op {
		case 'S':
			r = s.Scan()
			p := s.Pos()
			fmt.Fprintf(out, "S %s %s %s %d %s %d\n", scanner.TokenString(r), hex.EncodeToString([]byte(s.TokenText())), s.Position, s.Position.Offset, p, p.Offset)
		case 'N':
			r = s.Next()
			p := s.Pos()
			fmt.Fprintf(out, "N %s %s %q %s %d\n", scanner.TokenString(r), s.Position, s.TokenText(), p, p.Offset)
		case 'P':
			fmt.Fprintf(out, "P %s\n", scanner.TokenString(s.Peek()))
		case 'M':
			s.Mode ^= scanner.ScanStrings | scanner.ScanComments | scanner.SkipComments
		case 'W':
			s.Whitespace ^= 1 << '\n'
		}
		flush()
		return r
	}
	done := false
	for n := 0; n < 100000 && !done; n++ {
		op := ops[n%len(ops)]
		if step(op) == scanner.EOF && (op == 'S' || op == 'N') {
			done = true
		}
	}
	step('S')
	step('N')
	step('P')
	fmt.Fprintf(out, "X %d\n", s.ErrorCount)
}
