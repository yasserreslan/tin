// Command crucible prints what Go's testing/iotest, testing/quick, testing/fstest and crypto/subtle
// say for a fixed set of cases, and one line per comparison of the cryptotest corpus on stdin.
// tools/ci/fixtures/crucible.tin prints the same lines with the helpers of toolchain/std/crucible,
// and tools/ci/crucible_check.tin compares them (#920).
//
//	C <hex|-> <hex|->     crypto/subtle.ConstantTimeCompare of the two buffers ("-" is empty): 1 or 0
package main

import (
	"bufio"
	"bytes"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"testing/fstest"
	"testing/iotest"
	"testing/quick"
)

// src reads a string and returns (0, nil) at its end, as Tin's readers do.
type src struct {
	s  string
	at int
}

func (r *src) Read(p []byte) (int, error) {
	n := copy(p, r.s[r.at:])
	r.at += n
	return n, nil
}

// one makes one read of size bytes and describes it.
func one(r io.Reader, size int) string {
	buf := make([]byte, size)
	n, err := r.Read(buf)
	if err != nil {
		return "err=" + err.Error()
	}
	return fmt.Sprintf("n=%d data=%s", n, buf[:n])
}

// lines makes one read of size bytes and describes the data and the error it came with, one line each.
func lines(r io.Reader, size int) []string {
	buf := make([]byte, size)
	n, err := r.Read(buf)
	out := []string{}
	if n > 0 || err == nil {
		out = append(out, fmt.Sprintf("n=%d data=%s", n, buf[:n]))
	}
	if err != nil {
		out = append(out, "err="+err.Error())
	}
	return out
}

// outcome is "ok" for a nil error and its message otherwise.
func outcome(err error) string {
	if err == nil {
		return "ok"
	}
	return err.Error()
}

// names lists the entries as name(kind), joined by commas.
func names(entries []fs.DirEntry) string {
	parts := []string{}
	for _, e := range entries {
		kind := "file"
		if e.IsDir() {
			kind = "dir"
		}
		parts = append(parts, e.Name()+"("+kind+")")
	}
	return strings.Join(parts, ",")
}

// readChunks reads the named file in 3-byte reads, joining the chunks with "|".
func readChunks(fsys fs.FS, p string) (string, error) {
	f, err := fsys.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	parts := []string{}
	buf := make([]byte, 3)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			parts = append(parts, string(buf[:n]))
		}
		if err != nil {
			break
		}
	}
	return strings.Join(parts, "|"), nil
}

// dirChunks lists the named directory in ReadDir(2) chunks, joining them with "|" (an end is an empty chunk).
func dirChunks(fsys fs.FS, p string) (string, error) {
	d, err := fsys.Open(p)
	if err != nil {
		return "", err
	}
	defer d.Close()
	rd := d.(fs.ReadDirFile)
	parts := []string{}
	for i := 0; i < 3; i++ {
		entries, _ := rd.ReadDir(2)
		parts = append(parts, names(entries))
	}
	return strings.Join(parts, "|"), nil
}

// shortStat is MapFS whose Stat reports one byte more than the contents hold.
type shortStat struct {
	fstest.MapFS
}

// bigger is a FileInfo with one byte more.
type bigger struct {
	fs.FileInfo
}

func (b bigger) Size() int64 { return b.FileInfo.Size() + 1 }

func (s shortStat) Stat(name string) (fs.FileInfo, error) {
	info, err := s.MapFS.Stat(name)
	if err != nil {
		return nil, err
	}
	return bigger{info}, nil
}

// testFS is "ok" for a nil fstest.TestFS error and "error" otherwise.
func testFS(fsys fs.FS, expected ...string) string {
	if fstest.TestFS(fsys, expected...) == nil {
		return "ok"
	}
	return "error"
}

// roundTrip is true when strconv gives back x.
func roundTrip(x int64) bool {
	v, err := strconv.Atoi(strconv.FormatInt(x, 10))
	return err == nil && int64(v) == x
}

func main() {
	// iotest: the same reads over the same source.
	t := iotest.TimeoutReader(&src{s: "hello world"})
	fmt.Println("timeout", one(t, 4), one(t, 4), one(t, 4), one(t, 4), one(t, 4))
	e := iotest.TimeoutReader(&src{s: ""})
	fmt.Println("timeout empty", one(e, 8), one(e, 8), one(e, 8))
	h := iotest.HalfReader(&src{s: "hello world"})
	fmt.Println("half", one(h, 3), one(h, 3), one(h, 3), one(h, 3), one(h, 3), one(h, 3), one(h, 3))
	// Go's DataErrReader returns the last data with io.EOF in one call; Tin's returns the data first and its
	// fault on the next call. Both print the data line and then the error line, here for one read.
	d := iotest.DataErrReader(strings.NewReader("abc"))
	fmt.Println("dataerr", strings.Join(lines(d, 8), " "))
	var sinkBuf bytes.Buffer
	w := iotest.TruncateWriter(&sinkBuf, 7)
	for _, p := range []string{"abc", "defgh", "ij"} {
		n, err := w.Write([]byte(p))
		fmt.Println("write", n, outcome(err), "got", sinkBuf.String())
	}

	// quick: a round trip over int64, a counted run, a property that never holds, and the generators.
	fmt.Println("quick roundtrip", outcome(quick.Check(roundTrip, nil)))
	calls := 0
	fmt.Println("quick count", outcome(quick.Check(func(x int64) bool { calls++; return true }, &quick.Config{MaxCount: 25})), calls)
	calls = 0
	never := quick.Check(func(s string) bool { return false }, nil)
	fmt.Println("quick fail", strings.Split(outcome(never), ":")[0], "calls", calls)
	fmt.Println("quick check2", outcome(quick.Check(func(b bool, s string) bool { return len(s) < 1000 }, nil)))
	fmt.Println("quick float", outcome(quick.Check(func(x float64) bool { return x-x == 0 }, nil)))
	fmt.Println("quick bytes", outcome(quick.Check(func(b []byte) bool { return len(b) < 1000 }, nil)))
	fmt.Println("quick str", outcome(quick.Check(func(s string) bool { return len(s) < 1000 }, nil)))
	fmt.Println("quick uint", outcome(quick.Check(func(x uint64) bool { return x == x }, &quick.Config{MaxCount: 7})))
	fmt.Println("quick equal", outcome(quick.CheckEqual(func(s string) int { return len(s) }, func(s string) int { return len(s) }, nil)))
	neq := quick.CheckEqual(func(s string) int { return len(s) }, func(s string) int { return len(s) + 1 }, nil)
	fmt.Println("quick unequal", strings.Split(outcome(neq), ":")[0])

	// fstest: the MapFS of the strict test, walked by fs.WalkDir and read through Open, ReadDir, Stat and Glob.
	fsys := fstest.MapFS{
		"a.txt":         {Data: []byte("alpha")},
		"dir/b.txt":     {Data: []byte("bravo!")},
		"dir/empty":     {Mode: fs.ModeDir | 0o755},
		"dir/sub/c.txt": {Data: []byte("charlie")},
	}
	walk := fs.WalkDir(fsys, ".", func(p string, de fs.DirEntry, err error) error {
		kind := "file"
		if de.IsDir() {
			kind = "dir"
		}
		fmt.Println("walk", p, kind)
		return nil
	})
	fmt.Println("walk", outcome(walk))
	text, _ := fs.ReadFile(fsys, "dir/b.txt")
	fmt.Println("readfile", string(text), "ok")
	listed, lerr := fs.ReadDir(fsys, "dir")
	fmt.Println("readdir", names(listed), outcome(lerr))
	info, ierr := fs.Stat(fsys, "dir/sub/c.txt")
	fmt.Println("stat", info.Name(), info.Size(), info.IsDir(), outcome(ierr))
	sub, serr := fs.Stat(fsys, "dir/sub")
	fmt.Println("stat dir", sub.Name(), sub.IsDir(), outcome(serr))
	globbed, gerr := fs.Glob(fsys, "dir/*.txt")
	fmt.Println("glob", strings.Join(globbed, ","), outcome(gerr))
	chunks, cerr := readChunks(fsys, "dir/sub/c.txt")
	fmt.Println("chunks", chunks, outcome(cerr))
	dchunks, derr := dirChunks(fsys, "dir")
	fmt.Println("dir chunks", dchunks, outcome(derr))
	_, merr := fs.Stat(fsys, "missing")
	fmt.Println("missing", merr != nil)
	fmt.Println("testfs", testFS(fsys, "a.txt", "dir/sub/c.txt"))
	fmt.Println("testfs missing", testFS(fsys, "nope.txt"))
	fmt.Println("testfs size", testFS(shortStat{fsys}))

	// cryptotest: the comparison corpus on stdin, through crypto/subtle.
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		fields := strings.Fields(in.Text())
		if len(fields) != 3 || fields[0] != "C" {
			continue
		}
		a := decode(fields[1])
		b := decode(fields[2])
		fmt.Fprintln(out, "C", subtle.ConstantTimeCompare(a, b))
	}
}

// decode is the bytes of a hex field, "-" being empty.
func decode(s string) []byte {
	if s == "-" {
		return []byte{}
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
