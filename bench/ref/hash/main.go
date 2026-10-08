// Command hash reads a hex corpus on stdin and prints, for each case, what Go's hash/crc64 and
// compress/lzw say. tools/ci/fixtures/hash.tin prints the same lines and tools/ci/hash_check.py
// compares them (#754).
//
//	P <hex>            crc64: ECMA and ISO of the data, and the same chained over two halves
//	Z <order> <hex>    lzw: the encoding of the data and the decoding of that encoding
package main

import (
	"bufio"
	"bytes"
	"compress/lzw"
	"encoding/hex"
	"fmt"
	"hash/crc64"
	"io"
	"os"
	"strings"
)

var ecma = crc64.MakeTable(crc64.ECMA)
var iso = crc64.MakeTable(crc64.ISO)

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
		// An empty data field makes the line have one field fewer, so the arguments are read by
		// position with a default.
		arg := func(i int) string {
			if i < len(fields) {
				return fields[i]
			}
			return ""
		}
		switch fields[0] {
		case "P":
			raw := decodeHex(arg(1))
			half := len(raw) / 2
			e := crc64.Update(crc64.Update(0, ecma, raw[:half]), ecma, raw[half:])
			i := crc64.Update(crc64.Update(0, iso, raw[:half]), iso, raw[half:])
			fmt.Fprintf(out, "P %016x %016x %016x %016x\n",
				crc64.Checksum(raw, ecma), crc64.Checksum(raw, iso), e, i)
		case "Z":
			raw := decodeHex(arg(2))
			order := lzw.MSB
			if arg(1) == "1" {
				order = lzw.LSB
			}
			var coded bytes.Buffer
			w := lzw.NewWriter(&coded, order, 8)
			w.Write(raw)
			w.Close()
			r := lzw.NewReader(bytes.NewReader(coded.Bytes()), order, 8)
			back, err := io.ReadAll(r)
			r.Close()
			if err != nil {
				fmt.Fprintf(out, "Z %s fault\n", hex.EncodeToString(coded.Bytes()))
				continue
			}
			fmt.Fprintf(out, "Z %s %s\n", hex.EncodeToString(coded.Bytes()), hex.EncodeToString(back))
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
