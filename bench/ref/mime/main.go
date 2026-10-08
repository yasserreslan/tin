// Command mime reads a hex corpus on stdin and prints, for each case, what Go's mime,
// mime/quotedprintable and mime/multipart say. tools/ci/fixtures/mime.tin prints the same lines
// and tools/ci/mime_check.py compares them (#735).
//
//	P <hex>              a media type string: the parsed type and parameters
//	Q <hex>              text: quoted-printable decode, and the encoding of the decoded text
//	M <boundary-hex> <body-hex>   a multipart body: its parts (name, filename, type, body)
package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"os"
	"strings"
)

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		fields := strings.Split(in.Text(), " ")
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "P":
			raw := decodeHex(fields[1])
			t, params, err := mime.ParseMediaType(raw)
			if err != nil {
				fmt.Fprintln(out, "P fault")
				continue
			}
			list := []string{}
			for _, k := range sortedKeys(params) {
				list = append(list, k+"="+params[k])
			}
			fmt.Fprintf(out, "P %s %s\n", t, strings.Join(list, "&"))
		case "Q":
			raw := decodeHex(fields[1])
			dec, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(raw)))
			if err != nil {
				fmt.Fprintln(out, "Q fault")
				continue
			}
			var enc strings.Builder
			w := quotedprintable.NewWriter(&enc)
			w.Write(dec)
			w.Close()
			fmt.Fprintf(out, "Q %s %s\n", hex.EncodeToString(dec), hex.EncodeToString([]byte(enc.String())))
		case "M":
			boundary := decodeHex(fields[1])
			body := decodeHex(fields[2])
			r := multipart.NewReader(strings.NewReader(body), boundary)
			parts := []string{}
			for {
				p, err := r.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					parts = append(parts, "fault")
					break
				}
				data, err := io.ReadAll(p)
				if err != nil {
					parts = append(parts, "fault")
					break
				}
				parts = append(parts, fmt.Sprintf("%s|%s|%s|%s", p.FormName(), p.FileName(), p.Header.Get("Content-Type"), hex.EncodeToString(data)))
			}
			fmt.Fprintf(out, "M %s\n", strings.Join(parts, " "))
		}
	}
}

func decodeHex(s string) string {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func sortedKeys(m map[string]string) []string {
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
