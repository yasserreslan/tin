package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
)

func pixels(m image.Image) string {
	var b bytes.Buffer
	r := m.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			r, g, bl, a := m.At(x, y).RGBA()
			fmt.Fprintf(&b, "%04x%04x%04x%04x", r, g, bl, a)
		}
	}
	return b.String()
}

func encode(m image.Image) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, m); err != nil {
		panic(err)
	}
	return b.Bytes()
}

func corpus() []image.Image {
	gray := image.NewGray(image.Rect(0, 0, 3, 2))
	gray.Pix = []byte{0, 64, 255, 17, 128, 240}
	gray16 := image.NewGray16(image.Rect(0, 0, 3, 1))
	gray16.Pix = []byte{0x12, 0x34, 0xab, 0xcd, 0xff, 0xff}
	nrgba := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for i := range nrgba.Pix {
		nrgba.Pix[i] = byte(i*37 + 9)
	}
	nrgba64 := image.NewNRGBA64(image.Rect(0, 0, 3, 1))
	for i := range nrgba64.Pix {
		nrgba64.Pix[i] = byte(i*19 + 3)
	}
	rgba := image.NewRGBA(image.Rect(0, 0, 3, 1))
	rgba.SetRGBA(0, 0, color.RGBA{R: 120, G: 50, B: 20, A: 128})
	rgba.SetRGBA(1, 0, color.RGBA{R: 17, G: 80, B: 99, A: 255})
	rgba.SetRGBA(2, 0, color.RGBA{R: 1, G: 2, B: 3, A: 0})
	rgba64 := image.NewRGBA64(image.Rect(0, 0, 1, 3))
	rgba64.SetRGBA64(0, 0, color.RGBA64{R: 0x1234, G: 0x5678, B: 0x9876, A: 0xffff})
	rgba64.SetRGBA64(0, 1, color.RGBA64{R: 0x1010, G: 0x2020, B: 0x3030, A: 0x8080})
	rgba64.SetRGBA64(0, 2, color.RGBA64{})
	pal := image.NewPaletted(image.Rect(0, 0, 5, 1), color.Palette{color.RGBA{R: 255, A: 255}, color.RGBA{G: 255, A: 255}})
	pal.Pix = []byte{0, 1, 0, 1, 0}
	return []image.Image{gray, gray16, nrgba, nrgba64, rgba, rgba64, pal}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "draw" {
		for i := 0; i < 4; i++ {
			fmt.Println(drawResult(i))
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "malformed" {
		base := encode(corpus()[0])
		truncated := append([]byte(nil), base[:len(base)-7]...)
		badCRC := append([]byte(nil), base...)
		badCRC[29] ^= 1
		badZlib := append([]byte(nil), base...)
		mutateChunk(badZlib, "IDAT", func(p []byte) { p[0] ^= 0xff })
		interlaced := append([]byte(nil), base...)
		interlaced[28] = 1
		rewriteChunkCRC(interlaced, 8, 13)
		for i, p := range [][]byte{truncated, badCRC, badZlib, interlaced} {
			fmt.Printf("bad%d %s\n", i, hex.EncodeToString(p))
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "verifybad" {
		for _, line := range bytes.Split(bytes.TrimSpace(readStdin()), []byte{'\n'}) {
			f := bytes.Split(line, []byte{' '})
			p, err := hex.DecodeString(string(f[0]))
			if err != nil {
				panic(err)
			}
			if _, err = png.Decode(bytes.NewReader(p)); err == nil {
				panic("Go accepted malformed PNG")
			}
		}
		fmt.Println("OK")
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "verify" {
		for i, line := range bytes.Split(bytes.TrimSpace(readStdin()), []byte{'\n'}) {
			f := bytes.Split(line, []byte{' '})
			if len(f) != 2 {
				panic("bad verify record")
			}
			p, err := hex.DecodeString(string(f[0]))
			if err != nil {
				panic(err)
			}
			want, err := hex.DecodeString(string(f[1]))
			if err != nil {
				panic(err)
			}
			m, err := png.Decode(bytes.NewReader(p))
			if err != nil {
				panic(err)
			}
			got, _ := hex.DecodeString(pixels(m))
			if !bytes.Equal(got, want) {
				panic(fmt.Sprintf("Tin PNG pixels differ from Go source at %d: got %x want %x", i+1, got, want))
			}
		}
		fmt.Println("OK")
		return
	}
	for _, m := range corpus() {
		b := encode(m)
		decoded, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s %s\n", hex.EncodeToString(b), pixels(decoded))
	}
	for _, b := range rawCorpus() {
		decoded, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s %s\n", hex.EncodeToString(b), pixels(decoded))
	}
}

func rawCorpus() [][]byte {
	var out [][]byte
	for _, spec := range [][2]int{{0, 1}, {0, 2}, {0, 4}, {0, 8}, {0, 16}, {4, 8}, {4, 16}, {2, 8}, {2, 16}, {6, 8}, {6, 16}, {3, 1}, {3, 2}, {3, 4}, {3, 8}} {
		out = append(out, rawPNG(spec[0], spec[1], 5, 3, true))
	}
	// Exercise degenerate and long scanline shapes in addition to the odd-width corpus.
	out = append(out, rawPNG(0, 8, 1, 1, false), rawPNG(2, 8, 31, 1, false), rawPNG(6, 16, 1, 23, false))
	return out
}

func rawPNG(typ, depth, w, h int, transparency bool) []byte {
	channels := map[int]int{0: 1, 2: 3, 3: 1, 4: 2, 6: 4}[typ]
	max := (1 << depth) - 1
	if depth == 16 {
		max = 65535
	}
	var raw bytes.Buffer
	for y := 0; y < h; y++ {
		row := make([]byte, (w*channels*depth+7)/8)
		for x := 0; x < w; x++ {
			for c := 0; c < channels; c++ {
				v := (x*3 + y*2 + c + 1) % (max + 1)
				if typ == 4 && c == 1 || typ == 6 && c == 3 {
					v = max
				}
				if depth == 16 {
					at := (x*channels + c) * 2
					row[at], row[at+1] = byte(v>>8), byte(v)
				} else if depth == 8 {
					row[x*channels+c] = byte(v)
				} else {
					bit := (x*channels + c) * depth
					shift := 8 - depth - (bit % 8)
					row[bit/8] |= byte(v << shift)
				}
			}
		}
		raw.WriteByte(0)
		raw.Write(row)
	}
	var ihdr bytes.Buffer
	binary.Write(&ihdr, binary.BigEndian, uint32(w))
	binary.Write(&ihdr, binary.BigEndian, uint32(h))
	ihdr.WriteByte(byte(depth))
	ihdr.WriteByte(byte(typ))
	ihdr.Write([]byte{0, 0, 0})
	var out bytes.Buffer
	out.Write([]byte{137, 80, 78, 71, 13, 10, 26, 10})
	writeChunk(&out, "IHDR", ihdr.Bytes())
	if typ == 3 {
		n := 1 << depth
		var plte, trns bytes.Buffer
		for i := 0; i < n; i++ {
			plte.WriteByte(byte(i))
			plte.WriteByte(byte(255 - i))
			plte.WriteByte(byte(i * 31))
			if transparency {
				trns.WriteByte(byte(i * 255 / (n - 1)))
			}
		}
		writeChunk(&out, "PLTE", plte.Bytes())
		if transparency {
			writeChunk(&out, "tRNS", trns.Bytes())
		}
	} else if transparency && (typ == 0 || typ == 2) {
		var trns bytes.Buffer
		for c := 0; c < channels; c++ {
			v := 1
			if typ == 2 {
				v = c + 1
			}
			if depth == 16 {
				binary.Write(&trns, binary.BigEndian, uint16(v))
			} else {
				binary.Write(&trns, binary.BigEndian, uint16(v))
			}
		}
		writeChunk(&out, "tRNS", trns.Bytes())
	}
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	zw.Write(raw.Bytes())
	zw.Close()
	writeChunk(&out, "IDAT", compressed.Bytes())
	writeChunk(&out, "IEND", nil)
	return out.Bytes()
}

func writeChunk(w *bytes.Buffer, name string, data []byte) {
	binary.Write(w, binary.BigEndian, uint32(len(data)))
	w.WriteString(name)
	w.Write(data)
	binary.Write(w, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(name), data...)))
}

func drawResult(which int) string {
	dst := image.NewRGBA(image.Rect(0, 0, 4, 3))
	src := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	mask := image.NewAlpha(image.Rect(0, 0, 4, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			dst.SetRGBA(x, y, color.RGBA{R: byte(20 + x*19), G: byte(30 + y*21), B: 70, A: 255})
			src.SetNRGBA(x, y, color.NRGBA{R: byte(200 - x*23), G: byte(40 + y*17), B: byte(10 + x*9), A: byte(60 + x*43 + y*11)})
			mask.SetAlpha(x, y, color.Alpha{A: byte(30 + x*50 + y*13)})
		}
	}
	r := image.Rect(-1, 0, 3, 3)
	if which == 0 {
		draw.Draw(dst, r, src, image.Pt(0, 0), draw.Src)
	}
	if which == 1 {
		draw.Draw(dst, r, src, image.Pt(0, 0), draw.Over)
	}
	if which == 2 {
		draw.DrawMask(dst, r, src, image.Pt(0, 0), mask, image.Pt(0, 0), draw.Src)
	}
	if which == 3 {
		draw.DrawMask(dst, r, src, image.Pt(0, 0), mask, image.Pt(0, 0), draw.Over)
	}
	return pixels(dst)
}

func mutateChunk(p []byte, name string, f func([]byte)) {
	for at := 8; at+12 <= len(p); {
		n := int(binary.BigEndian.Uint32(p[at : at+4]))
		if at+12+n > len(p) {
			panic("bad chunk")
		}
		if string(p[at+4:at+8]) == name {
			f(p[at+8 : at+8+n])
			rewriteChunkCRC(p, at, n)
			return
		}
		at += 12 + n
	}
	panic("missing chunk")
}

func rewriteChunkCRC(p []byte, at, n int) {
	binary.BigEndian.PutUint32(p[at+8+n:at+12+n], crc32.ChecksumIEEE(p[at+4:at+8+n]))
}

func readStdin() []byte {
	b, err := os.ReadFile("/dev/stdin")
	if err != nil {
		panic(err)
	}
	return b
}
