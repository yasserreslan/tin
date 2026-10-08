// Command pack reads a hex corpus on stdin and prints, for each case, what Go's encoding/binary,
// encoding/base32 and encoding/ascii85 say. tools/ci/fixtures/pack.tin prints the same lines and
// tools/ci/pack_check.tin compares them (#740).
//
// Lines:
//
//	P <hex64>   a u64 value: the packed forms
//	U <hex>     bytes: the unpacked u64 reads and the varints
//	B <hex>     bytes: base32 (standard and hex) and ascii85 encodings, a decode of the raw
//	            bytes as a string, and the round trip of the encodings
package main

import (
	"bufio"
	"encoding/ascii85"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		line := in.Text()
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, " ", 2)
		raw, err := hex.DecodeString(fields[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "hex:", err)
			os.Exit(1)
		}
		switch fields[0] {
		case "P":
			v := binary.BigEndian.Uint64(raw)
			var b2 [2]byte
			binary.LittleEndian.PutUint16(b2[:], uint16(v))
			le16 := hex.EncodeToString(b2[:])
			binary.BigEndian.PutUint16(b2[:], uint16(v))
			be16 := hex.EncodeToString(b2[:])
			var b4 [4]byte
			binary.LittleEndian.PutUint32(b4[:], uint32(v))
			le32 := hex.EncodeToString(b4[:])
			binary.BigEndian.PutUint32(b4[:], uint32(v))
			be32 := hex.EncodeToString(b4[:])
			var b8 [8]byte
			binary.LittleEndian.PutUint64(b8[:], v)
			le64 := hex.EncodeToString(b8[:])
			binary.BigEndian.PutUint64(b8[:], v)
			be64 := hex.EncodeToString(b8[:])
			var uv [binary.MaxVarintLen64]byte
			n := binary.PutUvarint(uv[:], v)
			uvarint := hex.EncodeToString(uv[:n])
			n = binary.PutVarint(uv[:], int64(v))
			sv := hex.EncodeToString(uv[:n])
			fmt.Fprintf(out, "pack %d %s %s %s %s %s %s %s %s\n", v, le16, be16, le32, be32, le64, be64, uvarint, sv)
		case "U":
			var b8 [8]byte
			copy(b8[:], raw)
			u, n := binary.Uvarint(raw)
			uvar := "fault"
			if n > 0 {
				uvar = fmt.Sprintf("%d", u)
			}
			sv, sn := binary.Varint(raw)
			svar := "fault"
			if sn > 0 {
				svar = fmt.Sprintf("%d", sv)
			}
			fmt.Fprintf(out, "unpack %d %d %s %s\n",
				binary.LittleEndian.Uint64(b8[:]), binary.BigEndian.Uint64(b8[:]), uvar, svar)
		case "B":
			std := base32.StdEncoding.EncodeToString(raw)
			hexA := base32.HexEncoding.EncodeToString(raw)
			a85buf := make([]byte, ascii85.MaxEncodedLen(len(raw)))
			nA := ascii85.Encode(a85buf, raw)
			a85 := string(a85buf[:nA])
			fmt.Fprintf(out, "encoding %s %s %s\n", std, hexA, a85)
			// The raw bytes as a string: the decoders get arbitrary input, valid or not.
			text := string(raw)
			stdDec, errS := base32.StdEncoding.DecodeString(text)
			stdD := hex.EncodeToString(stdDec)
			if errS != nil {
				stdD = "fault"
			}
			hexDec, errH := base32.HexEncoding.DecodeString(text)
			hexD := hex.EncodeToString(hexDec)
			if errH != nil {
				hexD = "fault"
			}
			// 'z' expands one byte into four, so the destination must be generous: Decode
			// silently stops when it runs out.
			dec := make([]byte, 4*len(raw)+8)
			nd, _, errD := ascii85.Decode(dec, raw, true)
			a85D := hex.EncodeToString(dec[:nd])
			if errD != nil {
				a85D = "fault"
			}
			fmt.Fprintf(out, "decode %s %s %s\n", stdD, hexD, a85D)
			// And the round trip of this case's own encodings.
			r1, e1 := base32.StdEncoding.DecodeString(std)
			r2, e2 := base32.HexEncoding.DecodeString(hexA)
			r3buf := make([]byte, 4*len(a85buf[:nA])+8)
			n3, _, e3 := ascii85.Decode(r3buf, a85buf[:nA], true)
			r1h, r2h, r3h := hex.EncodeToString(r1), hex.EncodeToString(r2), hex.EncodeToString(r3buf[:n3])
			if e1 != nil {
				r1h = "fault"
			}
			if e2 != nil {
				r2h = "fault"
			}
			if e3 != nil {
				r3h = "fault"
			}
			fmt.Fprintf(out, "roundtrip %s %s %s\n", r1h, r2h, r3h)
		}
	}
	if err := in.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}
}
