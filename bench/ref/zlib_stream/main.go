// Go side of tools/ci/zlib_stream_check.tin (#772): compress/zlib against squash's streaming zlib.
//
//	go run ./bench/ref/zlib_stream files DIR   inflate DIR/tin-N.zlib against DIR/in-N.bin; write DIR/go-N.zlib
//	go run ./bench/ref/zlib_stream read N      inflate a zlib stream of gen(N) from stdin and check it
//	go run ./bench/ref/zlib_stream write N     write a zlib stream of gen(N) to stdout
package main

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// gen is the byte at position i of the streamed input, the same function as the Tin fixture's.
func gen(i int64) byte {
	if i%4096 == 0 {
		return byte((i / 4096) % 251)
	}
	return byte(97 + (i*7+i/13)%26)
}

type genReader struct{ at, n int64 }

func (g *genReader) Read(p []byte) (int, error) {
	if g.at >= g.n {
		return 0, io.EOF
	}
	k := int64(len(p))
	if g.n-g.at < k {
		k = g.n - g.at
	}
	for i := int64(0); i < k; i++ {
		p[i] = gen(g.at + i)
	}
	g.at += k
	return int(k), nil
}

func main() {
	switch os.Args[1] {
	case "files":
		dir := os.Args[2]
		names, _ := filepath.Glob(filepath.Join(dir, "in-*.bin"))
		sort.Strings(names)
		ok, bad := 0, 0
		for i, name := range names {
			n := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(name), "in-"), ".bin")
			in, err := os.ReadFile(name)
			if err != nil {
				panic(err)
			}
			z, err := os.ReadFile(filepath.Join(dir, "tin-"+n+".zlib"))
			if err != nil {
				panic(err)
			}
			r, err := zlib.NewReader(bytes.NewReader(z))
			var out []byte
			if err == nil {
				out, err = io.ReadAll(r)
			}
			if err != nil || !bytes.Equal(out, in) {
				fmt.Println("go inflates tin size", n, "FAIL", err)
				bad++
			} else {
				ok++
			}
			var b bytes.Buffer
			w, _ := zlib.NewWriterLevel(&b, []int{1, 6, 9}[i%3])
			w.Write(in)
			w.Close()
			if err := os.WriteFile(filepath.Join(dir, "go-"+n+".zlib"), b.Bytes(), 0o644); err != nil {
				panic(err)
			}
		}
		fmt.Println("go inflates tin:", ok, "ok,", bad, "failed")
	case "read":
		n, _ := strconv.ParseInt(os.Args[2], 10, 64)
		r, err := zlib.NewReader(bufio.NewReaderSize(os.Stdin, 1<<20))
		if err != nil {
			fmt.Println("go reads tin's stream FAIL", err)
			return
		}
		br := bufio.NewReaderSize(r, 1<<20)
		var at int64
		for {
			c, err := br.ReadByte()
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Println("go reads tin's stream FAIL at", at, err)
				return
			}
			if c != gen(at) {
				fmt.Println("go reads tin's stream FAIL: byte", at, "differs")
				return
			}
			at++
		}
		if at != n {
			fmt.Println("go reads tin's stream FAIL:", at, "bytes, not", n)
			return
		}
		fmt.Println("go reads tin's stream of", n, "bytes ok")
	case "write":
		n, _ := strconv.ParseInt(os.Args[2], 10, 64)
		out := bufio.NewWriterSize(os.Stdout, 1<<20)
		w, _ := zlib.NewWriterLevel(out, 6)
		io.CopyBuffer(w, &genReader{n: n}, make([]byte, 1<<20))
		w.Close()
		out.Flush()
	}
}
