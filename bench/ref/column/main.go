// Command column reads a hex corpus on stdin and prints, for each case, what Go's text/tabwriter says.
// tools/ci/fixtures/column.tin prints the same lines and tools/ci/column_check.tin compares them (#753).
//
//	T <flags> <minwidth> <tabwidth> <padding> <padchar> <text>          one Write, then Flush
//	W <flags> <minwidth> <tabwidth> <padding> <padchar> <chunk> ...     one Write per chunk, then Flush
package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
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
		flags, _ := strconv.ParseInt(arg(1), 10, 64)
		minwidth, _ := strconv.ParseInt(arg(2), 10, 64)
		tabwidth, _ := strconv.ParseInt(arg(3), 10, 64)
		padding, _ := strconv.ParseInt(arg(4), 10, 64)
		padchar := byte(0)
		if raw := decodeHex(arg(5)); len(raw) == 1 {
			padchar = raw[0]
		}
		var b bytes.Buffer
		w := tabwriter.NewWriter(&b, int(minwidth), int(tabwidth), int(padding), padchar, uint(flags))
		if fields[0] == "T" {
			w.Write(decodeHex(arg(6)))
		} else {
			for i := 6; i < len(fields); i++ {
				w.Write(decodeHex(fields[i]))
			}
		}
		w.Flush()
		fmt.Fprintf(out, "%s %s\n", fields[0], hex.EncodeToString(b.Bytes()))
	}
}

func decodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
