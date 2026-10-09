package main

// The htmltpl twin, Go side: read template, value kind and value triples (hex-encoded, tab separated, one per line) on stdin
// and print what html/template makes of each: the hex-encoded output, or PARSE or EXEC. tools/ci/htmltpl_check.tin feeds both
// sides the same corpus and compares the lines (#912).

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"html/template"
	"os"
	"strconv"
	"strings"
)

// value is the data of one kind, as the Tin side builds it: the trusted types and the plain kinds.
func value(kind string, s string) (any, bool) {
	switch kind {
	case "str":
		return s, true
	case "html":
		return template.HTML(s), true
	case "js":
		return template.JS(s), true
	case "jsstr":
		return template.JSStr(s), true
	case "css":
		return template.CSS(s), true
	case "url":
		return template.URL(s), true
	case "int":
		n, err := strconv.ParseInt(s, 10, 64)
		return n, err == nil
	case "bool":
		return s == "true", true
	case "none":
		return nil, true
	}
	return nil, false
}

func main() {
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	for {
		line, err := in.ReadString('\n')
		line = strings.TrimRight(line, "\n")
		if line != "" {
			f := strings.Split(line, "\t")
			src, _ := hex.DecodeString(f[0])
			val, _ := hex.DecodeString(f[2])
			data, ok := value(f[1], string(val))
			t, perr := template.New("t").Parse(string(src))
			if perr != nil {
				fmt.Println("PARSE")
			} else if !ok {
				fmt.Println("EXEC")
			} else {
				var sb strings.Builder
				if xerr := t.Execute(&sb, data); xerr != nil {
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
