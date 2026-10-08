// Command path reads a hex corpus on stdin and prints, for each case, what Go's path package says.
// tools/ci/fixtures/path.tin prints the same lines and tools/ci/path_check.tin compares them (#753):
// trail's Clean, Base, Dir, Ext, Split, IsAbs, Join and Match must agree with Go's path rules.
//
//	C <hex>              Clean
//	B <hex>              Base
//	D <hex>              Dir
//	E <hex>              Ext
//	S <hex>              Split: dir and file
//	A <hex>              IsAbs
//	J <h1> <h2> ...      Join of the parts
//	M <pattern> <name>   Match: true, false or bad (ErrBadPattern)
package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"strings"
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
				return string(decodeHex(fields[i]))
			}
			return ""
		}
		switch fields[0] {
		case "C":
			fmt.Fprintf(out, "C %s\n", hex.EncodeToString([]byte(path.Clean(arg(1)))))
		case "B":
			fmt.Fprintf(out, "B %s\n", hex.EncodeToString([]byte(path.Base(arg(1)))))
		case "D":
			fmt.Fprintf(out, "D %s\n", hex.EncodeToString([]byte(path.Dir(arg(1)))))
		case "E":
			fmt.Fprintf(out, "E %s\n", hex.EncodeToString([]byte(path.Ext(arg(1)))))
		case "S":
			dir, file := path.Split(arg(1))
			fmt.Fprintf(out, "S %s %s\n", hex.EncodeToString([]byte(dir)), hex.EncodeToString([]byte(file)))
		case "A":
			fmt.Fprintf(out, "A %t\n", path.IsAbs(arg(1)))
		case "J":
			parts := []string{}
			for i := 1; i < len(fields); i++ {
				parts = append(parts, string(decodeHex(fields[i])))
			}
			fmt.Fprintf(out, "J %s\n", hex.EncodeToString([]byte(path.Join(parts...))))
		case "M":
			ok, err := path.Match(arg(1), arg(2))
			if err != nil {
				fmt.Fprint(out, "M bad\n")
			} else {
				fmt.Fprintf(out, "M %t\n", ok)
			}
		}
	}
}

func decodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
