package main

import (
	"debug/elf"
	"debug/dwarf"
	"errors"
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: dwarf_twin ELF")
	}
	f, err := elf.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	d, err := f.DWARF()
	if err != nil {
		panic(err)
	}
	r := d.Reader()
	cu, err := r.Next()
	if err != nil || cu == nil || cu.Tag != dwarf.TagCompileUnit {
		panic("missing compilation unit")
	}
	lr, err := d.LineReader(cu)
	if err != nil || lr == nil {
		panic("missing line table")
	}
	var entry dwarf.LineEntry
	for {
		err = lr.Next(&entry)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			panic(err)
		}
		file := ""
		if entry.File != nil {
			file = entry.File.Name
		}
		fmt.Printf("R\t%d\t%s\t%d\t%d\t%t\n", entry.Address, file, entry.Line, entry.Column, entry.EndSequence)
	}
	for i, file := range lr.Files() {
		if i == 0 || file == nil {
			continue
		}
		fmt.Printf("F\t%d\t%s\n", i, file.Name)
	}
}
