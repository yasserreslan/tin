package main

// The html twin, Go side: read documents from stdin and print their token streams with golang.org/x/net/html's Tokenizer, one
// token per line, in the format tools/ci/fixtures/html_twin.tin prints. Each document is framed as its decimal length, a newline
// and its bytes; it is tokenized twice, without and with AllowCDATA. The check (tools/ci/html_check.tin) feeds both sides the
// same documents and compares the lines.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// enc writes b with printable ASCII (and no backslash) as is and every other byte as \xHH.
func enc(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		if c >= 0x20 && c <= 0x7e && c != '\\' {
			sb.WriteByte(c)
		} else {
			fmt.Fprintf(&sb, "\\x%02x", c)
		}
	}
	return sb.String()
}

func main() {
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}
	pos := 0
	for doc := 0; pos < len(in); doc++ {
		nl := bytes.IndexByte(in[pos:], '\n')
		n, err := strconv.Atoi(string(in[pos : pos+nl]))
		if err != nil {
			fmt.Fprintln(os.Stderr, "frame:", err)
			os.Exit(1)
		}
		pos += nl + 1
		text := in[pos : pos+n]
		pos += n
		fmt.Printf("escape %s\n", enc([]byte(html.EscapeString(string(text)))))
		fmt.Printf("unescape %s\n", enc([]byte(html.UnescapeString(string(text)))))
		for _, cdata := range []bool{false, true} {
			name := "plain"
			if cdata {
				name = "cdata"
			}
			fmt.Printf("doc %d %s\n", doc, name)
			z := html.NewTokenizer(bytes.NewReader(text))
			z.AllowCDATA(cdata)
			for {
				tt := z.Next()
				if tt == html.ErrorToken {
					fmt.Printf("end %v\n", z.Err())
					break
				}
				raw := enc(z.Raw())
				t := z.Token()
				line := fmt.Sprintf("%s raw=%s data=%s", tt, raw, enc([]byte(t.Data)))
				for _, a := range t.Attr {
					line += " attr=" + enc([]byte(a.Key)) + ":" + enc([]byte(a.Val))
				}
				fmt.Println(line)
			}
		}
	}
}
