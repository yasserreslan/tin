// The jsontext twin, Go side: read one case per line on stdin and print one line for it, so tools/ci/jsontext_check.tin can compare
// it with toolchain/std/jsontext (#931). A case is a mode, then its operand:
//
//	T <hex>   the tokens of the text, then the end: EOF, or the decoder's error
//	V <hex>   the raw text of each value, then the end
//	W<n> <hex> the tokens of the text written with encoder options n (see options), or the decoder's or encoder's error
//	F <hex16> jsontext.Float of the float64 with those bits, written
//	I <dec>   jsontext.Int of that integer, written
//
// Each output line joins its items with '|'. Tokens are kind:hex(text); errors are E:hex(message); hex is lower case.
package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json/jsontext"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

// options returns the encoder options of case n.
func options(n int) []jsontext.Options {
	switch n {
	case 1:
		return []jsontext.Options{jsontext.WithIndent("  ")}
	case 2:
		return []jsontext.Options{jsontext.Multiline(true)}
	case 3:
		return []jsontext.Options{jsontext.SpaceAfterColon(true), jsontext.SpaceAfterComma(true)}
	case 4:
		return []jsontext.Options{jsontext.EscapeForHTML(true), jsontext.EscapeForJS(true)}
	case 5:
		return []jsontext.Options{jsontext.WithIndent("\t"), jsontext.WithIndentPrefix("  ")}
	}
	return nil
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<22), 1<<22)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		fields := strings.SplitN(in.Text(), " ", 2)
		if len(fields) != 2 {
			continue
		}
		mode, arg := fields[0], fields[1]
		raw, _ := hex.DecodeString(arg)
		var items []string
		switch {
		case mode == "T":
			d := jsontext.NewDecoder(strings.NewReader(string(raw)))
			for {
				t, err := d.ReadToken()
				if err == io.EOF {
					items = append(items, "EOF")
					break
				}
				if err != nil {
					items = append(items, "E:"+hex.EncodeToString([]byte(err.Error())))
					break
				}
				items = append(items, string(t.Kind())+":"+hex.EncodeToString([]byte(t.String())))
			}
		case mode == "V":
			d := jsontext.NewDecoder(strings.NewReader(string(raw)))
			for {
				v, err := d.ReadValue()
				if err == io.EOF {
					items = append(items, "EOF")
					break
				}
				if err != nil {
					items = append(items, "E:"+hex.EncodeToString([]byte(err.Error())))
					break
				}
				items = append(items, hex.EncodeToString(v))
			}
		case strings.HasPrefix(mode, "W"):
			n, _ := strconv.Atoi(mode[1:])
			var sb strings.Builder
			e := jsontext.NewEncoder(&sb, options(n)...)
			d := jsontext.NewDecoder(strings.NewReader(string(raw)))
			status := ""
			for {
				t, err := d.ReadToken()
				if err == io.EOF {
					break
				}
				if err != nil {
					status = "D:" + hex.EncodeToString([]byte(err.Error()))
					break
				}
				if err := e.WriteToken(t); err != nil {
					status = "X:" + hex.EncodeToString([]byte(err.Error()))
					break
				}
			}
			if status == "" {
				status = hex.EncodeToString([]byte(sb.String()))
			}
			items = append(items, status)
		case mode == "F":
			bits, _ := strconv.ParseUint(arg, 16, 64)
			var sb strings.Builder
			e := jsontext.NewEncoder(&sb)
			status := hex.EncodeToString([]byte(sb.String()))
			if err := e.WriteToken(jsontext.Float(math.Float64frombits(bits))); err != nil {
				status = "X:" + hex.EncodeToString([]byte(err.Error()))
			} else {
				status = hex.EncodeToString([]byte(sb.String()))
			}
			items = append(items, status)
		case mode == "I":
			n, _ := strconv.ParseInt(arg, 10, 64)
			var sb strings.Builder
			e := jsontext.NewEncoder(&sb)
			_ = e.WriteToken(jsontext.Int(n))
			items = append(items, hex.EncodeToString([]byte(sb.String())))
		}
		fmt.Fprintln(out, strings.Join(items, "|"))
	}
}
