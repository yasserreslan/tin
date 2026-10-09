package main

import (
	"debug/elf"
	"fmt"
	"os"
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
}
