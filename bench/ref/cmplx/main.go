// Command cmplx reads a corpus of math/cmplx calls on stdin and prints what Go's math/cmplx says for each. Every float is
// the hex of its bits, except that a NaN prints as "nan": the payload of a NaN is not part of the result (Go's NaN() and
// the propagated ones differ across the operations that make them). tools/ci/fixtures/cmplx.tin prints the same lines and
// tools/ci/cmplx_check.tin compares them (#916). On amd64 the check builds this with Go's portable Exp and Log (see there).
//
//	<op> <re> <im> [<re2> <im2>]   a complex argument; ops with one complex argument read two fields
//	<op> <r> <theta>               Rect reads two real fields
//	<op> <re> <im>                 the complex results print "re im"; the real results print one field
//	Inf | NaN                      the constants
//	C64 <re> <im>                  complex64(z), printed as the float32 bits of each part (8 hex digits)
//	C64SQRT <re> <im>              Sqrt of complex128(complex64(z)), rounded to complex64 and printed the same way
package main

import (
	"bufio"
	"fmt"
	"math"
	"math/cmplx"
	"os"
	"strconv"
	"strings"
)

// f64 parses a field of hex bits.
func f64(s string) float64 {
	b, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		panic(err)
	}
	return math.Float64frombits(b)
}

// out formats one float: its bits, or "nan".
func out(x float64) string {
	if math.IsNaN(x) {
		return "nan"
	}
	return fmt.Sprintf("%016x", math.Float64bits(x))
}

// out32 formats one float32: its bits, or "nan".
func out32(x float32) string {
	if math.IsNaN(float64(x)) {
		return "nan"
	}
	return fmt.Sprintf("%08x", math.Float32bits(x))
}

// c formats a complex result.
func c(z complex128) string {
	return out(real(z)) + " " + out(imag(z))
}

// b formats a bool result.
func b(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// unary maps a complex function name to Go's function.
var unary = map[string]func(complex128) complex128{
	"Sqrt": cmplx.Sqrt, "Exp": cmplx.Exp, "Log": cmplx.Log, "Log10": cmplx.Log10,
	"Asin": cmplx.Asin, "Acos": cmplx.Acos, "Atan": cmplx.Atan,
	"Asinh": cmplx.Asinh, "Acosh": cmplx.Acosh, "Atanh": cmplx.Atanh,
	"Sin": cmplx.Sin, "Cos": cmplx.Cos, "Tan": cmplx.Tan, "Cot": cmplx.Cot,
	"Sinh": cmplx.Sinh, "Cosh": cmplx.Cosh, "Tanh": cmplx.Tanh, "Conj": cmplx.Conj,
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	for in.Scan() {
		f := strings.Fields(in.Text())
		if len(f) == 0 {
			continue
		}
		op := f[0]
		var line string
		switch {
		case op == "Inf":
			line = c(cmplx.Inf())
		case op == "NaN":
			line = c(cmplx.NaN())
		case op == "Pow":
			line = c(cmplx.Pow(complex(f64(f[1]), f64(f[2])), complex(f64(f[3]), f64(f[4]))))
		case op == "Rect":
			line = c(cmplx.Rect(f64(f[1]), f64(f[2])))
		case op == "Abs":
			line = out(cmplx.Abs(complex(f64(f[1]), f64(f[2]))))
		case op == "Phase":
			line = out(cmplx.Phase(complex(f64(f[1]), f64(f[2]))))
		case op == "Polar":
			r, t := cmplx.Polar(complex(f64(f[1]), f64(f[2])))
			line = out(r) + " " + out(t)
		case op == "IsNaN":
			line = b(cmplx.IsNaN(complex(f64(f[1]), f64(f[2]))))
		case op == "IsInf":
			line = b(cmplx.IsInf(complex(f64(f[1]), f64(f[2]))))
		case op == "C64":
			z := complex64(complex(f64(f[1]), f64(f[2])))
			line = out32(real(z)) + " " + out32(imag(z))
		case op == "C64SQRT":
			z := complex64(complex(f64(f[1]), f64(f[2])))
			r := complex64(cmplx.Sqrt(complex128(z)))
			line = out32(real(r)) + " " + out32(imag(r))
		default:
			fn, ok := unary[op]
			if !ok {
				panic("unknown op " + op)
			}
			line = c(fn(complex(f64(f[1]), f64(f[2]))))
		}
		fmt.Fprintln(w, op, line)
	}
}
