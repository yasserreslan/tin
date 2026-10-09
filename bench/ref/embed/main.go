// Command embed prints, for each pattern, the file tree it selects from the fixture tree tools/ci/fixtures/embed_tree, with Go's
// rules: each element of the pattern is a name taken as it is or a glob matched with path.Match, a directory is walked with its
// hidden entries (names starting with . or _) left out, and the selected files form a testing/fstest.MapFS that fs.WalkDir
// lists. tools/ci/fixtures/embed.tin embeds the same patterns with its compiler directive and prints the same lines, and
// tools/ci/embed_check.tin compares them (#928). Run from the repository root.
//
//	pattern <P>
//	d <path>                    a directory, as fs.WalkDir visits it
//	f <path> <size> <hex>       a file with its size and its bytes in hex
package main

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"testing/fstest"
)

// fixtures is the directory of tools/ci/fixtures/embed.tin: the patterns are relative to it.
const fixtures = "tools/ci/fixtures"

// patterns are the directive patterns of tools/ci/fixtures/embed.tin, in the same order.
var patterns = []string{
	"embed_tree/*",
	"embed_tree/*.txt",
	"embed_tree/sub",
	"embed_tree/a?b*",
	"embed_tree/README.md",
	"embed_tree/*/deep",
}

func hidden(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

// walk adds every regular file below the directory rel (slash-separated, relative to the fixture directory) to out.
func walk(rel string, out map[string][]byte) {
	entries, err := os.ReadDir(path.Join(fixtures, rel))
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		if hidden(e.Name()) {
			continue
		}
		child := path.Join(rel, e.Name())
		if e.IsDir() {
			walk(child, out)
			continue
		}
		data, err := os.ReadFile(path.Join(fixtures, child))
		if err != nil {
			panic(err)
		}
		out[child] = data
	}
}

// selectFiles adds the files a pattern selects from the directory rel to out; elems are the pattern's remaining elements.
func selectFiles(rel string, elems []string, out map[string][]byte) {
	el := elems[0]
	last := len(elems) == 1
	if !strings.ContainsAny(el, "*?") {
		// a name is taken as it is
		child := path.Join(rel, el)
		info, err := os.Stat(path.Join(fixtures, child))
		if err != nil {
			return
		}
		if info.IsDir() {
			if last {
				walk(child, out)
			} else {
				selectFiles(child, elems[1:], out)
			}
			return
		}
		if last {
			data, err := os.ReadFile(path.Join(fixtures, child))
			if err != nil {
				panic(err)
			}
			out[child] = data
		}
		return
	}
	entries, err := os.ReadDir(path.Join(fixtures, rel))
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		if hidden(e.Name()) {
			continue
		}
		if ok, _ := path.Match(el, e.Name()); !ok {
			continue
		}
		child := path.Join(rel, e.Name())
		if e.IsDir() {
			if last {
				walk(child, out)
			} else {
				selectFiles(child, elems[1:], out)
			}
		} else if last {
			data, err := os.ReadFile(path.Join(fixtures, child))
			if err != nil {
				panic(err)
			}
			out[child] = data
		}
	}
}

func main() {
	for _, p := range patterns {
		fmt.Println("pattern", p)
		files := map[string][]byte{}
		selectFiles("", strings.Split(p, "/"), files)
		mfs := fstest.MapFS{}
		for name, data := range files {
			mfs[name] = &fstest.MapFile{Data: data}
		}
		err := fs.WalkDir(mfs, ".", func(name string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				fmt.Printf("d %s\n", name)
				return nil
			}
			data, err := fs.ReadFile(mfs, name)
			if err != nil {
				return err
			}
			fmt.Printf("f %s %d %s\n", name, len(data), hex.EncodeToString(data))
			return nil
		})
		if err != nil {
			panic(err)
		}
	}
}
