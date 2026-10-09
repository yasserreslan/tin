package main

import (
	"debug/macho"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// packed formats a version packed as xxxx.yy.zz the way Tin's buildinfo does.
func packed(v uint32) string {
	return fmt.Sprintf("%d.%d.%d", v>>16, (v>>8)&0xff, v&0xff)
}

func main() {
	f, err := macho.Open(os.Args[1])
	if err != nil {
		_, format := err.(*macho.FormatError)
		fmt.Printf("rejected %t\n", format)
		return
	}
	defer f.Close()
	fmt.Printf("header %d %d %d %d %d %d\n", f.Cpu, f.SubCpu, f.Type, f.Ncmd, f.Cmdsz, f.Flags)
	var uuid, build, entry []byte
	var entryValue uint64
	var buildValue [4]uint32
	for _, load := range f.Loads {
		raw := load.Raw()
		cmd := binary.LittleEndian.Uint32(raw[0:4])
		fmt.Printf("command %d %d\n", cmd, len(raw))
		switch cmd {
		case 0x1b:
			uuid = raw[8:24]
		case 0x32:
			build = raw
			for i := range buildValue {
				buildValue[i] = binary.LittleEndian.Uint32(raw[8+4*i:])
			}
		case 0x80000028:
			entry = raw
			entryValue = binary.LittleEndian.Uint64(raw[8:16])
		}
	}
	for _, load := range f.Loads {
		if s, ok := load.(*macho.Segment); ok {
			fmt.Printf("segment %s %d %d %d %d %d %d %d %d\n", s.Name, s.Addr, s.Memsz, s.Offset, s.Filesz, s.Maxprot, s.Prot, s.Nsect, s.Flag)
		}
	}
	for _, s := range f.Sections {
		fmt.Printf("section %s %s %d %d %d %d %d %d %d\n", s.Seg, s.Name, s.Addr, s.Size, s.Offset, s.Align, s.Reloff, s.Nreloc, s.Flags)
	}
	libs, _ := f.ImportedLibraries()
	for _, lib := range libs {
		fmt.Printf("dylib %s\n", lib)
	}
	if uuid == nil {
		fmt.Println("uuid none")
	} else {
		fmt.Printf("uuid %s\n", hex.EncodeToString(uuid))
	}
	if build == nil {
		fmt.Println("build 0 0 0 0")
	} else {
		fmt.Printf("build %d %d %d %d\n", buildValue[0], buildValue[1], buildValue[2], buildValue[3])
	}
	if entry == nil {
		fmt.Println("entry 0")
	} else {
		fmt.Printf("entry %d\n", entryValue)
	}
	if f.Symtab == nil {
		fmt.Println("symbols 0")
	} else {
		fmt.Printf("symbols %d\n", len(f.Symtab.Syms))
		for _, s := range f.Symtab.Syms {
			fmt.Printf("symbol %s %d %d %d %d\n", s.Name, s.Type, s.Sect, s.Desc, s.Value)
		}
	}
	for _, s := range f.Sections {
		if s.Name != "__text" && s.Name != "__cstring" && s.Name != "__const" {
			continue
		}
		data, err := s.Data()
		if err != nil {
			_, format := err.(*macho.FormatError)
			fmt.Printf("section-data rejected %t\n", format)
			return
		}
		head := data
		if len(head) > 8 {
			head = head[:8]
		}
		fmt.Printf("section-data %s.%s %d %s\n", s.Seg, s.Name, len(data), hex.EncodeToString(head))
	}
	if uuid == nil || buildValue[0] == 0 {
		fmt.Println("buildinfo none true")
		return
	}
	fmt.Printf("buildinfo %s %d %s %s %d\n", formatUUID(uuid), buildValue[0], packed(buildValue[1]), packed(buildValue[2]), len(libs))
}

// formatUUID writes the 16 bytes as 8-4-4-4-12 upper-case hex.
func formatUUID(b []byte) string {
	s := hex.EncodeToString(b)
	return strings.ToUpper(fmt.Sprintf("%s-%s-%s-%s-%s", s[0:8], s[8:12], s[12:16], s[16:20], s[20:32]))
}
