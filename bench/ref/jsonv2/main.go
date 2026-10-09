// The jsonv2 twin, Go side: read one case per line on stdin and print one line for it, so tools/ci/jsonv2_check.tin can compare it
// with packages/jsonv2 (#931). A case is a type and its document in hex:
//
//	N <hex>  Unmarshal into Note (Title, Tags, At, Ok), then Marshal under four option sets
//	P <hex>  Unmarshal into Point (X, Y), then Marshal under the same four
//
// The four Marshal results are hex joined with '|': no options; WithIndent("  "); Multiline and SpaceAfterColon; EscapeForHTML and
// EscapeForJS. A document that does not decode is "E", and a Marshal that fails is "X" for the whole line.
package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"os"
	"strings"
)

type Point struct {
	X int64
	Y int64
}

type Note struct {
	Title string
	Tags  []string
	Labels map[string]string
	At    *Point
	Ok    bool
}

func marshalAll(v any) string {
	sets := [][]jsontext.Options{
		nil,
		{jsontext.WithIndent("  ")},
		{jsontext.Multiline(true), jsontext.SpaceAfterColon(true)},
		{jsontext.EscapeForHTML(true), jsontext.EscapeForJS(true)},
	}
	var items []string
	for _, opts := range sets {
		out, err := jsonv2.Marshal(v, opts...)
		if err != nil {
			return "X"
		}
		items = append(items, hex.EncodeToString(out))
	}
	return strings.Join(items, "|")
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
		raw, _ := hex.DecodeString(fields[1])
		var line string
		switch fields[0] {
		case "N":
			var n Note
			if err := jsonv2.Unmarshal(raw, &n); err != nil {
				line = "E"
			} else {
				line = marshalAll(n)
			}
		case "P":
			var p Point
			if err := jsonv2.Unmarshal(raw, &p); err != nil {
				line = "E"
			} else {
				line = marshalAll(p)
			}
		}
		fmt.Fprintln(out, line)
	}
}
