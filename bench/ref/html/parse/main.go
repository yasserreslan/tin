package main

// The html parse twin, Go side: read documents from stdin (framed as in bench/ref/html) and print the tree that html.Parse
// builds, one node per line, indented two spaces per level, in the format tools/ci/fixtures/html_parse_twin.tin prints. A document
// that fails to parse prints "error". The check (tools/ci/html_check.tin) compares the lines.

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
func enc(b string) string {
	var sb strings.Builder
	for _, c := range []byte(b) {
		if c >= 0x20 && c <= 0x7e && c != '\\' {
			sb.WriteByte(c)
		} else {
			fmt.Fprintf(&sb, "\\x%02x", c)
		}
	}
	return sb.String()
}

// walk prints the children of n, one level deeper than depth.
func walk(n *html.Node, depth int) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		indent := strings.Repeat("  ", depth)
		switch c.Type {
		case html.TextNode:
			fmt.Printf("%s\"%s\"\n", indent, enc(c.Data))
		case html.CommentNode:
			fmt.Printf("%s<!--%s-->\n", indent, enc(c.Data))
		case html.DoctypeNode:
			line := indent + "<!DOCTYPE " + enc(c.Data)
			for _, a := range c.Attr {
				line += " " + a.Key + "=\"" + enc(a.Val) + "\""
			}
			fmt.Println(line + ">")
		case html.ElementNode:
			name := c.Data
			if c.Namespace != "" {
				name = c.Namespace + " " + c.Data
			}
			fmt.Printf("%s<%s>\n", indent, enc(name))
			for _, a := range c.Attr {
				key := a.Key
				if a.Namespace != "" {
					key = a.Namespace + ":" + a.Key
				}
				fmt.Printf("%s  %s=\"%s\"\n", indent, enc(key), enc(a.Val))
			}
			walk(c, depth+1)
		}
	}
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
		fmt.Printf("doc %d\n", doc)
		root, err := html.Parse(bytes.NewReader(text))
		if err != nil {
			fmt.Println("error")
			continue
		}
		walk(root, 0)
	}
}
