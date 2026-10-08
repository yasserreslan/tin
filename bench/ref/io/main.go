// Command io reads a corpus of chunk patterns on stdin and prints, for each case, what Go's io
// helpers do with a reader that returns the chunks. tools/ci/fixtures/io.tin prints the same
// lines and tools/ci/io_check.py compares them (#736).
//
// The reader yields the bytes of a deterministic sequence (byte i is (i*7+3) % 251) in the chunk
// sizes of the corpus line, which are comma-separated. Cases:
//
//	R <chunks>          ReadAll, and Copy into a writer, with buffer sizes 1, 3 and 7
//	L <chunks> <n>      LimitReader(n) then ReadAll
//	F <chunks> <n>      ReadFull(n) and ReadAtLeast(n) results
//	N <chunks> <n>      CopyN(n)
//	T <chunks>          TeeReader into a second writer, ReadAll
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func seqByte(i int) byte { return byte((i*7 + 3) % 251) }

type chunked struct {
	chunks []int
	at     int
	pos    int
}

func (c *chunked) Read(p []byte) (int, error) {
	if c.at >= len(c.chunks) {
		return 0, io.EOF
	}
	n := c.chunks[c.at]
	if n > len(p) {
		n = len(p)
	}
	for i := 0; i < n; i++ {
		p[i] = seqByte(c.pos + i)
	}
	c.pos += n
	if n == c.chunks[c.at] {
		c.at++
	} else {
		c.chunks[c.at] -= n
	}
	return n, nil
}

func parseChunks(s string) []int {
	out := []int{}
	for _, f := range strings.Split(s, ",") {
		v, _ := strconv.Atoi(f)
		out = append(out, v)
	}
	return out
}

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
		chunks := parseChunks(fields[1])
		switch fields[0] {
		case "R":
			r := &chunked{chunks: append([]int{}, chunks...)}
			data, err := io.ReadAll(r)
			if err != nil {
				fmt.Fprintln(out, "R fault", err)
				continue
			}
			sums := []string{}
			for _, size := range []int{1, 3, 7} {
				r2 := &chunked{chunks: append([]int{}, chunks...)}
				var dst strings.Builder
				n, err := io.CopyBuffer(&dst, r2, make([]byte, size))
				if err != nil {
					sums = append(sums, "fault")
					continue
				}
				sums = append(sums, fmt.Sprintf("%d/%d", n, dst.Len()))
			}
			fmt.Fprintf(out, "R %d/%d %s\n", len(data), checksum(data), strings.Join(sums, " "))
		case "L":
			k, _ := strconv.Atoi(fields[2])
			r := &chunked{chunks: append([]int{}, chunks...)}
			data, err := io.ReadAll(io.LimitReader(r, int64(k)))
			if err != nil {
				fmt.Fprintln(out, "L fault", err)
				continue
			}
			fmt.Fprintf(out, "L %d/%d\n", len(data), checksum(data))
		case "F":
			k, _ := strconv.Atoi(fields[2])
			r := &chunked{chunks: append([]int{}, chunks...)}
			buf := make([]byte, k)
			n, err := io.ReadFull(r, buf)
			full := fmt.Sprintf("%d/%d", n, checksum(buf[:n]))
			if err != nil {
				full = "fault"
			}
			r2 := &chunked{chunks: append([]int{}, chunks...)}
			buf2 := make([]byte, k)
			n2, err2 := io.ReadAtLeast(r2, buf2, k)
			least := fmt.Sprintf("%d/%d", n2, checksum(buf2[:n2]))
			if err2 != nil {
				least = "fault"
			}
			fmt.Fprintf(out, "F %s %s\n", full, least)
		case "N":
			k, _ := strconv.Atoi(fields[2])
			r := &chunked{chunks: append([]int{}, chunks...)}
			var dst strings.Builder
			n, err := io.CopyN(&dst, r, int64(k))
			res := fmt.Sprintf("%d/%d", n, dst.Len())
			if err != nil {
				res = fmt.Sprintf("fault/%d", dst.Len())
			}
			fmt.Fprintf(out, "N %s\n", res)
		case "T":
			r := &chunked{chunks: append([]int{}, chunks...)}
			var tee strings.Builder
			data, err := io.ReadAll(io.TeeReader(r, &tee))
			if err != nil {
				fmt.Fprintln(out, "T fault", err)
				continue
			}
			fmt.Fprintf(out, "T %d/%d %d/%d\n", len(data), checksum(data), tee.Len(), checksum([]byte(tee.String())))
		}
	}
	if err := in.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}
}

// checksum is a small rolling hash so the lines stay short but order matters.
func checksum(data []byte) uint64 {
	var h uint64
	for _, b := range data {
		h = h*1000003 + uint64(b)
	}
	return h
}
