// archive/tar interoperability oracle for tools/ci/tar_check.tin.
package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const largeSize = 32 << 20

// entries are the file entries generate writes and verify reads back, in order; the global header is only in generate.
func entries() []*tar.Header {
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
		{Name: strings.Repeat("a", 150), Mode: 0600, Uid: 5, Gid: 6, Typeflag: tar.TypeReg},
		{Name: "pax-link", Mode: 0777, Typeflag: tar.TypeSymlink, Linkname: strings.Repeat("l", 130)},
		{Name: "pax-owner", Mode: 0600, Uid: 2097152, Gid: 2097153, Uname: strings.Repeat("u", 40), Gname: strings.Repeat("g", 33), Typeflag: tar.TypeReg},
		{Name: "pax-mtime", Mode: 0600, ModTime: time.Unix(1700000001, 500000000).UTC(), Typeflag: tar.TypeReg, Format: tar.FormatPAX},
		{Name: "pax-mtime-nano", Mode: 0600, ModTime: time.Unix(1700000002, 123456789).UTC(), Typeflag: tar.TypeReg, Format: tar.FormatPAX},
		{Name: strings.Repeat("n", 150), Mode: 0600, Typeflag: tar.TypeReg, Format: tar.FormatGNU},
		{Name: "gnu-link", Mode: 0777, Typeflag: tar.TypeSymlink, Linkname: strings.Repeat("m", 150), Format: tar.FormatGNU},
		{Name: "large.bin", Mode: 0600, Size: largeSize, Typeflag: tar.TypeReg},
	}
}

func unix(sec int64) (t time.Time) { return time.Unix(sec, 0).UTC() }

func main() {
	if len(os.Args) != 3 { panic("usage: tar_oracle generate|verify|bad path") }
	switch os.Args[1] {
	case "generate": generate(os.Args[2])
	case "verify": verify(os.Args[2])
	case "bad": bad(os.Args[2])
	default: panic("unknown mode")
	}
}

func generate(path string) {
	f, err := os.Create(path); must(err)
	w := tar.NewWriter(f)
	for _, h := range entries() {
		if h.Name == "device-char" {
			must(w.WriteHeader(&tar.Header{Name: "global", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": "global header"}}))
		}
		must(w.WriteHeader(h))
		if h.Name == "regular.txt" { _, err = w.Write([]byte("hello")); must(err) }
		if h.Name == "large.bin" { writePattern(w, largeSize) }
	}
	must(w.Close()); must(f.Close())
}

func verify(path string) {
	f, err := os.Open(path); must(err)
	r := tar.NewReader(f)
	for i, want := range entries() {
		h, err := r.Next(); must(err)
		for h.Typeflag == tar.TypeXGlobalHeader { h, err = r.Next(); must(err) }
		wsec, wnsec := mtimeOf(want.ModTime)
		hsec, hnsec := mtimeOf(h.ModTime)
		if h.Name != want.Name || h.Mode != want.Mode || h.Uid != want.Uid || h.Gid != want.Gid || h.Size != want.Size || h.Typeflag != want.Typeflag || h.Linkname != want.Linkname || h.Uname != want.Uname || h.Gname != want.Gname || h.Devmajor != want.Devmajor || h.Devminor != want.Devminor || hsec != wsec || hnsec != wnsec {
			panic(fmt.Sprintf("entry %d mismatch: got %#v want %#v", i, h, want))
		}
		if h.Size > 0 {
			if want.Name == "regular.txt" { b, err := io.ReadAll(r); must(err); if string(b) != "hello" { panic("regular payload mismatch") } } else { checkPattern(r, h.Size) }
		}
		fmt.Printf("%s %o %d %d %d %d %d %d %q %q %q %d %d\n", h.Name, h.Mode, h.Uid, h.Gid, h.Size, h.Typeflag, hsec, hnsec, h.Linkname, h.Uname, h.Gname, h.Devmajor, h.Devminor)
	}
	_, err = r.Next(); if err != io.EOF { panic(fmt.Sprintf("expected end, got %v", err)) }
	must(f.Close())
}

// bad writes archives that archive/tar rejects and that tar.Reader must reject too: malformed PAX and global records, and oversized ones.
func bad(dir string) {
	must(os.MkdirAll(dir, 0o755))
	write := func(name string, parts ...[]byte) { must(os.WriteFile(filepath.Join(dir, name), bytes.Join(parts, nil), 0o644)) }
	file := rawEntry("file", '0', nil)
	end := make([]byte, 1024)
	write("bad-length.tar", rawEntry("PaxHeaders", 'x', []byte("12 path=x\n")), file, end)
	write("bad-equals.tar", rawEntry("PaxHeaders", 'x', []byte(record("pathx"))), file, end)
	write("bad-uid.tar", rawEntry("PaxHeaders", 'x', []byte(record("uid=x"))), file, end)
	write("bad-mtime.tar", rawEntry("PaxHeaders", 'x', []byte(record("mtime=1.x"))), file, end)
	write("bad-global.tar", rawEntry("global", 'g', []byte(record("comment"))), file, end)
	write("bad-huge.tar", rawEntry("PaxHeaders", 'x', []byte(record("path="+strings.Repeat("a", 2<<20)))), file, end)
	write("bad-size.tar", rawEntry("PaxHeaders", 'x', []byte(record("size=99999999"))), file, end)
}

// record is one PAX record for kv (key=value): its decimal length counts the whole record.
func record(kv string) string {
	body := " " + kv + "\n"
	n := len(body)
	for {
		next := len(body) + len(strconv.Itoa(n))
		if next == n { return strconv.Itoa(n) + body }
		n = next
	}
}

// rawEntry is one header block of the typeflag and its payload, padded to whole blocks, with the payload size in the header.
func rawEntry(name string, typeflag byte, payload []byte) []byte {
	b := make([]byte, 512)
	copy(b[0:100], name)
	copy(b[100:107], "0000644")
	copy(b[108:115], "0000000")
	copy(b[116:123], "0000000")
	copy(b[124:135], fmt.Sprintf("%011o", len(payload)))
	copy(b[136:147], "00000000000")
	for i := 148; i < 156; i++ { b[i] = ' ' }
	b[156] = typeflag
	copy(b[257:262], "ustar")
	copy(b[263:265], "00")
	sum := 0
	for _, c := range b { sum += int(c) }
	copy(b[148:154], fmt.Sprintf("%06o", sum))
	b[154] = 0
	out := append([]byte{}, b...)
	out = append(out, payload...)
	if rem := len(payload) % 512; rem > 0 { out = append(out, make([]byte, 512-rem)...) }
	return out
}

// mtimeOf is a time as the archive stores it: whole seconds and nanoseconds, the zero time as the Unix epoch.
func mtimeOf(t time.Time) (int64, int) {
	if t.IsZero() { return 0, 0 }
	return t.Unix(), t.Nanosecond()
}

func writePattern(w io.Writer, n int64) { b := pattern(); for n > 0 { k := int64(len(b)); if k > n { k = n }; _, err := w.Write(b[:k]); must(err); n -= k } }
func checkPattern(r io.Reader, n int64) { b := make([]byte, 65536); offset := int64(0); for offset < n { k := int64(len(b)); if k > n-offset { k = n-offset }; _, err := io.ReadFull(r, b[:k]); must(err); for i := int64(0); i < k; i++ { if b[i] != pattern()[(offset+i)%17] { panic("large payload mismatch") } }; offset += k } }
func pattern() []byte { return []byte("0123456789abcdef\n") }
func must(err error) { if err != nil { panic(err) } }
