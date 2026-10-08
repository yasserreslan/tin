// Command iofs reads a corpus of file system and pipe cases on stdin and prints what Go's io/fs
// (over os.DirFS of the tree named by its argument) and io.Pipe do with each.
// tools/ci/fixtures/iofs.tin prints the same lines and tools/ci/iofs_check.tin compares them (#736).
//
// Each corpus line is a command and its arguments, every argument hex-encoded:
//
//	V <name>                 fs.ValidPath
//	S <name>, L <name>       Stat, Lstat (name, size for a regular file, mode)
//	D <name>                 fs.ReadDir (names and types)
//	F <name>                 fs.ReadFile (length and FNV-1a 64)
//	O <name>                 Open, Stat, io.Copy of the file, ReadDir(-1) sorted, Close
//	W <root> <mode> <name>   fs.WalkDir: mode "-" visits all; "skipdir", "skipall" or "stop"
//	                         returns SkipDir, SkipAll or a fault when the entry's name is <name>
//	G <pattern>              fs.Glob
//	P <chunks> <buf>         a writer task writes the chunks to an io.Pipe and closes it; the
//	                         reader copies with a buffer of <buf> bytes (0: io.ReadAll)
//	E <chunks> <msg>         the writer closes with an error of text <msg> after the chunks
//	R <chunks> <k>           the reader reads <k> bytes and closes; the writer's fault
package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"
)

func seqByte(i int) byte { return byte((i*7 + 3) % 251) }

type hasher struct {
	n int64
	h uint64
}

func newHasher() *hasher { return &hasher{h: 14695981039346656037} }

func (h *hasher) Write(p []byte) (int, error) {
	for _, c := range p {
		h.h ^= uint64(c)
		h.h *= 1099511628211
	}
	h.n += int64(len(p))
	return len(p), nil
}

func (h *hasher) String() string { return fmt.Sprintf("%d %016x", h.n, h.h) }

func sum(b []byte) string {
	h := newHasher()
	h.Write(b)
	return h.String()
}

func arg(s string) string {
	if s == "-" {
		return ""
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func errText(err error) string {
	if err == nil {
		return "nil"
	}
	return "fault " + err.Error()
}

func info(fi fs.FileInfo) string {
	size := "-"
	if fi.Mode().IsRegular() {
		size = strconv.FormatInt(fi.Size(), 10)
	}
	return fi.Name() + " " + size + " " + fi.Mode().String() + " " + strconv.FormatBool(fi.IsDir())
}

func chunks(s string) []int {
	out := []int{}
	for _, f := range strings.Split(s, ",") {
		v, _ := strconv.Atoi(f)
		out = append(out, v)
	}
	return out
}

func writeChunks(w *io.PipeWriter, cs []int) error {
	pos := 0
	for _, c := range cs {
		b := make([]byte, c)
		for i := range b {
			b[i] = seqByte(pos + i)
		}
		pos += c
		if _, err := w.Write(b); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	fsys := os.DirFS(os.Args[1])
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		f := strings.Fields(in.Text())
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "V":
			fmt.Fprintln(out, "V", fs.ValidPath(arg(f[1])))
		case "S", "L":
			var fi fs.FileInfo
			var err error
			if f[0] == "S" {
				fi, err = fs.Stat(fsys, arg(f[1]))
			} else {
				fi, err = fsys.(fs.ReadLinkFS).Lstat(arg(f[1]))
			}
			if err != nil {
				fmt.Fprintln(out, f[0], errText(err))
				continue
			}
			fmt.Fprintln(out, f[0], info(fi))
		case "D":
			es, err := fs.ReadDir(fsys, arg(f[1]))
			if err != nil {
				fmt.Fprintln(out, "D", errText(err))
				continue
			}
			parts := []string{}
			for _, e := range es {
				parts = append(parts, e.Name()+":"+e.Type().String())
			}
			fmt.Fprintln(out, "D", len(es), strings.Join(parts, "|"))
		case "F":
			b, err := fs.ReadFile(fsys, arg(f[1]))
			if err != nil {
				fmt.Fprintln(out, "F", errText(err))
				continue
			}
			fmt.Fprintln(out, "F", sum(b))
		case "O":
			file, err := fsys.Open(arg(f[1]))
			if err != nil {
				fmt.Fprintln(out, "O", errText(err))
				continue
			}
			fi, err := file.Stat()
			if err != nil {
				fmt.Fprintln(out, "O stat", errText(err))
			} else {
				fmt.Fprintln(out, "O stat", info(fi))
			}
			h := newHasher()
			_, err = io.Copy(h, struct{ io.Reader }{file})
			fmt.Fprintln(out, "O copy", h.String(), errText(err))
			if d, ok := file.(fs.ReadDirFile); ok {
				es, err := d.ReadDir(-1)
				names := []string{}
				for _, e := range es {
					names = append(names, e.Name())
				}
				sort.Strings(names)
				fmt.Fprintln(out, "O readdir", strings.Join(names, "|"), errText(err))
			}
			fmt.Fprintln(out, "O close", errText(file.Close()))
		case "W":
			root, mode, name := arg(f[1]), f[2], arg(f[3])
			stop := errors.New("stop at " + name)
			err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
				if d == nil {
					fmt.Fprintln(out, "W", p, "nil", errText(err))
				} else {
					fmt.Fprintln(out, "W", p, d.Name(), d.IsDir(), d.Type().String(), errText(err))
				}
				if d != nil && d.Name() == name && err == nil {
					switch mode {
					case "skipdir":
						return fs.SkipDir
					case "skipall":
						return fs.SkipAll
					case "stop":
						return stop
					}
				}
				return nil
			})
			fmt.Fprintln(out, "W end", errText(err))
		case "G":
			m, err := fs.Glob(fsys, arg(f[1]))
			if err != nil {
				fmt.Fprintln(out, "G", errText(err))
				continue
			}
			fmt.Fprintln(out, "G", len(m), strings.Join(m, "|"))
		case "P", "E", "R":
			cs := chunks(f[1])
			pr, pw := io.Pipe()
			werr := make(chan error, 1)
			go func() {
				err := writeChunks(pw, cs)
				if f[0] == "E" {
					pw.CloseWithError(errors.New(arg(f[2])))
				} else {
					pw.Close()
				}
				werr <- err
			}()
			switch f[0] {
			case "P", "E":
				n, _ := strconv.Atoi(f[len(f)-1])
				if f[0] == "E" {
					n = 0
				}
				h := newHasher()
				var err error
				if n == 0 {
					var b []byte
					b, err = io.ReadAll(pr)
					if err == nil {
						h.Write(b)
					}
				} else {
					_, err = io.CopyBuffer(h, struct{ io.Reader }{pr}, make([]byte, n))
				}
				pr.Close()
				if err != nil {
					fmt.Fprintln(out, f[0], errText(err), "writer", errText(<-werr))
				} else {
					fmt.Fprintln(out, f[0], h.String(), "writer", errText(<-werr))
				}
			case "R":
				k, _ := strconv.Atoi(f[2])
				b := make([]byte, k)
				_, err := io.ReadFull(pr, b)
				pr.Close()
				fmt.Fprintln(out, "R", sum(b), errText(err), "writer", errText(<-werr))
			}
		default:
			panic("unknown command " + f[0])
		}
	}
}
