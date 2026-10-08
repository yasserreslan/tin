// archive/tar interoperability oracle for tools/ci/tar_check.tin.
package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"time"
)

const largeSize = 32 << 20

func headers() []*tar.Header {
	boundary := append(bytes.Repeat([]byte{'p'}, 155), append([]byte("/"), bytes.Repeat([]byte{'n'}, 100)...)...)
	return []*tar.Header{
		{Name: "regular.txt", Mode: 0640, Uid: 12, Gid: 34, Size: 5, ModTime: unix(1700000000), Typeflag: tar.TypeReg, Uname: "user", Gname: "group"},
		{Name: "dir/", Mode: 0755, Uid: 1, Gid: 2, Typeflag: tar.TypeDir},
		{Name: "sym", Mode: 0777, Typeflag: tar.TypeSymlink, Linkname: "target"},
		{Name: "hard", Mode: 0644, Typeflag: tar.TypeLink, Linkname: "regular.txt"},
		{Name: "fifo", Mode: 0600, Typeflag: tar.TypeFifo},
		{Name: "device-char", Mode: 0600, Typeflag: tar.TypeChar, Devmajor: 8, Devminor: 1},
		{Name: "device-block", Mode: 0600, Typeflag: tar.TypeBlock, Devmajor: 8, Devminor: 2},
		{Name: string(boundary), Mode: 0600, Uid: 3, Gid: 4, Typeflag: tar.TypeReg},
		{Name: "large.bin", Mode: 0600, Size: largeSize, Typeflag: tar.TypeReg},
	}
}

func unix(sec int64) (t time.Time) { return time.Unix(sec, 0).UTC() }

func main() {
	if len(os.Args) != 3 { panic("usage: tar_oracle generate|verify path") }
	switch os.Args[1] {
	case "generate": generate(os.Args[2])
	case "verify": verify(os.Args[2])
	default: panic("unknown mode")
	}
}

func generate(path string) {
	f, err := os.Create(path); must(err)
	w := tar.NewWriter(f)
	for i, h := range headers() {
		must(w.WriteHeader(h))
		if i == 0 { _, err = w.Write([]byte("hello")); must(err) }
		if h.Name == "large.bin" { writePattern(w, largeSize) }
	}
	must(w.Close()); must(f.Close())
}

func verify(path string) {
	f, err := os.Open(path); must(err)
	r := tar.NewReader(f)
	for i, want := range headers() {
		h, err := r.Next(); must(err)
		if h.Name != want.Name || h.Mode != want.Mode || h.Uid != want.Uid || h.Gid != want.Gid || h.Size != want.Size || h.Typeflag != want.Typeflag || h.Linkname != want.Linkname || h.Uname != want.Uname || h.Gname != want.Gname || h.Devmajor != want.Devmajor || h.Devminor != want.Devminor {
			panic(fmt.Sprintf("entry %d mismatch: got %#v want %#v", i, h, want))
		}
		if h.Size > 0 {
			if i == 0 { b, err := io.ReadAll(r); must(err); if string(b) != "hello" { panic("regular payload mismatch") } } else { checkPattern(r, h.Size) }
		}
		fmt.Printf("%s %o %d %d %d %d %q %q %q %d %d\n", h.Name, h.Mode, h.Uid, h.Gid, h.Size, h.Typeflag, h.Linkname, h.Uname, h.Gname, h.Devmajor, h.Devminor)
	}
	_, err = r.Next(); if err != io.EOF { panic(fmt.Sprintf("expected end, got %v", err)) }
	must(f.Close())
}

func writePattern(w io.Writer, n int64) { b := pattern(); for n > 0 { k := int64(len(b)); if k > n { k = n }; _, err := w.Write(b[:k]); must(err); n -= k } }
func checkPattern(r io.Reader, n int64) { b := make([]byte, 65536); offset := int64(0); for offset < n { k := int64(len(b)); if k > n-offset { k = n-offset }; _, err := io.ReadFull(r, b[:k]); must(err); for i := int64(0); i < k; i++ { if b[i] != pattern()[(offset+i)%17] { panic("large payload mismatch") } }; offset += k } }
func pattern() []byte { return []byte("0123456789abcdef\n") }
func must(err error) { if err != nil { panic(err) } }
