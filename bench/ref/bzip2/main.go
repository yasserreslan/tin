// Command bzip2 reads file paths on stdin, one per line, and prints for each what Go's
// compress/bzip2 makes of the file: "ok <length> <sha256>" for the decompressed bytes, or
// "fault <message>". tools/ci/fixtures/bzip2.tin prints the same lines with squash.Bunzip2 and
// tools/ci/bzip2_check.tin compares them (#754).
package main

import (
	"bufio"
	"bytes"
	"compress/bzip2"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

func main() {
	in := bufio.NewScanner(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		path := in.Text()
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		back, err := io.ReadAll(bzip2.NewReader(bytes.NewReader(data)))
		if err == io.ErrUnexpectedEOF {
			fmt.Fprintln(out, "fault bzip2: unexpected EOF")
			continue
		}
		if err != nil {
			fmt.Fprintln(out, "fault "+err.Error())
			continue
		}
		fmt.Fprintf(out, "ok %d %x\n", len(back), sha256.Sum256(back))
	}
}
