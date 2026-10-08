package main

// The stencil URL twin, Go side: read template/value pairs (hex-encoded, tab separated, one pair per line) on stdin and print what
// html/template makes of each, as the hex-encoded output or "PARSE"/"EXEC". tools/ci/stencil_url_check.tin feeds both sides the same
// corpus and compares the lines (#813).

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"html/template"
	"os"
	"strings"
)

func main() {
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	for {
		line, err := in.ReadString('\n')
		line = strings.TrimRight(line, "\n")
		if line != "" {
			tab := strings.IndexByte(line, '\t')
			src, _ := hex.DecodeString(line[:tab])
			val, _ := hex.DecodeString(line[tab+1:])
			t, perr := template.New("t").Parse(string(src))
			if perr != nil {
				fmt.Println("PARSE")
			} else {
				var sb strings.Builder
				if xerr := t.Execute(&sb, map[string]string{"v": string(val)}); xerr != nil {
					fmt.Println("EXEC")
				} else {
					fmt.Println(hex.EncodeToString([]byte(sb.String())))
				}
			}
		}
		if err != nil {
			break
		}
	}
}
