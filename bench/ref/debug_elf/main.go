package main

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
)

func main() {
	f, err := elf.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	fmt.Printf("header %d %d %d %d\n", f.Class, f.Machine, len(f.Progs), len(f.Sections))
	for _, s := range f.Sections {
		fmt.Printf("section %s\n", s.Name)
	}
	var names []string
	if syms, err := f.Symbols(); err == nil {
		for _, s := range syms {
			names = append(names, s.Name)
		}
	}
	if syms, err := f.DynamicSymbols(); err == nil {
		for _, s := range syms {
			names = append(names, s.Name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Printf("symbol %s\n", name)
	}
	for _, s := range f.Sections {
		if s.Type != elf.SHT_DYNAMIC {
			continue
		}
		data, err := s.Data()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for p := 0; p+16 <= len(data); p += int(s.Entsize) {
			tag := int64(binary.LittleEndian.Uint64(data[p:]))
			value := binary.LittleEndian.Uint64(data[p+8:])
			fmt.Printf("dynamic %d %d\n", tag, value)
			if tag == 0 {
				break
			}
		}
	}
	if s := f.Section(".shstrtab"); s != nil {
		data, err := s.Data()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("section-data %s %d\n", s.Name, len(data))
	}
}
