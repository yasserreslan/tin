package main

// The multipart twin, Go side: read cases (hex boundary, hex body, read chunk size, ignored; tab separated, one per line) on stdin and print
// what mime/multipart makes of each: one line per part (form name, file name, content type, length and SHA-1 of its content) and
// a last line "ok" or "err" (and the number of parts read before it). tools/ci/multipart_check.tin feeds both sides the same cases
// and compares the lines (#823).

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"strconv"
	"strings"
)

func main() {
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	n := 0
	for {
		line, err := in.ReadString('\n')
		line = strings.TrimRight(line, "\n")
		if line != "" {
			f := strings.Split(line, "\t")
			boundary, _ := hex.DecodeString(f[0])
			body, _ := hex.DecodeString(f[1])
			chunk, _ := strconv.Atoi(f[2])
			run(n, string(boundary), body, chunk)
			n++
		}
		if err != nil {
			break
		}
	}
}

func run(n int, boundary string, body []byte, chunk int) {
	r := multipart.NewReader(bytes.NewReader(body), boundary)
	parts := 0
	for {
		p, err := r.NextRawPart()
		if err == io.EOF {
			fmt.Printf("%d end ok %d\n", n, parts)
			return
		}
		if err != nil {
			fmt.Printf("%d end err %d\n", n, parts)
			return
		}
		parts++
		h := sha1.New()
		total := 0
		buf := make([]byte, chunk)
		var rerr error
		for {
			k, err := p.Read(buf)
			h.Write(buf[:k])
			total += k
			if err == io.EOF {
				break
			}
			if err != nil {
				rerr = err
				break
			}
		}
		fmt.Printf("%d part %s %s %s %d %s\n", n, hex.EncodeToString([]byte(p.FormName())), hex.EncodeToString([]byte(p.FileName())), hex.EncodeToString([]byte(p.Header.Get("Content-Type"))), total, hex.EncodeToString(h.Sum(nil)))
		if rerr != nil {
			fmt.Printf("%d end err %d\n", n, parts)
			return
		}
	}
}
