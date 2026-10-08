package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"io"
	"os"
	"strings"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "malformed" {
		fmt.Println("badheader 00000000000000000000000000")
		fmt.Println("notrailer 47494638396101000100000000")
		fmt.Println("truncatedtable 47494638396101000100f0000000")
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "verifybad" { verifyBad(); return }
	if len(os.Args) > 1 && os.Args[1] == "verify" {
		verify()
		return
	}
	for _, g := range corpus() {
		var b bytes.Buffer
		if err := gif.EncodeAll(&b, g); err != nil { panic(err) }
		decoded, err := gif.DecodeAll(bytes.NewReader(b.Bytes())); if err != nil { panic(err) }
		fmt.Printf("%s %s\n", hex.EncodeToString(b.Bytes()), digest(decoded))
	}
	for _, im := range singles() {
		var b bytes.Buffer
		if err := gif.Encode(&b, im, nil); err != nil { panic(err) }
		got, err := gif.DecodeAll(bytes.NewReader(b.Bytes()))
		if err != nil { panic(err) }
		fmt.Printf("%s %s\n", hex.EncodeToString(b.Bytes()), digest(got))
	}
}

func verifyBad() {
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		b, err := hex.DecodeString(strings.TrimSpace(s.Text())); if err != nil { panic(err) }
		if _, err := gif.DecodeAll(bytes.NewReader(b)); err == nil { fmt.Println("BAD accepted"); return }
	}
	fmt.Println("OK")
}

func verify() {
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		f := strings.Fields(s.Text())
		if len(f) != 2 { panic("bad verify record") }
		b, err := hex.DecodeString(f[0]); if err != nil { panic(err) }
		g, err := gif.DecodeAll(bytes.NewReader(b)); if err != nil { panic(err) }
		if got := digest(g); got != f[1] { fmt.Printf("BAD %s\n", got); return }
	}
	if err := s.Err(); err != nil && err != io.EOF { panic(err) }
	fmt.Println("OK")
}

func corpus() []*gif.GIF {
	p0 := color.Palette{color.NRGBA{R: 240, G: 10, B: 20, A: 255}, color.NRGBA{G: 230, A: 255}, color.NRGBA{B: 220, A: 0}, color.NRGBA{R: 1, G: 2, B: 3, A: 255}}
	p1 := color.Palette{color.NRGBA{R: 3, G: 4, B: 5, A: 255}, color.NRGBA{R: 210, G: 120, B: 30, A: 255}, color.NRGBA{A: 0}}
	a := frame(image.Rect(0, 0, 5, 3), p0, 0)
	b := frame(image.Rect(1, 1, 4, 3), p1, 1)
	c := frame(image.Rect(0, 0, 5, 3), p0, 2)
	return []*gif.GIF{
		{Image: []*image.Paletted{a}, Delay: []int{0}, Disposal: []byte{gif.DisposalNone}, LoopCount: -1},
		{Image: []*image.Paletted{a, b}, Delay: []int{7, 19}, Disposal: []byte{gif.DisposalBackground, gif.DisposalPrevious}, LoopCount: 4},
		{Image: []*image.Paletted{c, b, a}, Delay: []int{1, 250, 3}, Disposal: []byte{1, 2, 3}, LoopCount: 0},
	}
}

func singles() []*image.Paletted {
	p := color.Palette{color.NRGBA{R: 90, G: 130, B: 250, A: 255}, color.NRGBA{R: 30, B: 80, A: 255}, color.NRGBA{A: 0}}
	return []*image.Paletted{frame(image.Rect(0, 0, 1, 1), p, 1), frame(image.Rect(0, 0, 2, 7), p, 2)}
}

func frame(r image.Rectangle, p color.Palette, seed int) *image.Paletted {
	m := image.NewPaletted(r, p)
	for y := r.Min.Y; y < r.Max.Y; y++ { for x := r.Min.X; x < r.Max.X; x++ { m.SetColorIndex(x, y, uint8((x*3+y*5+seed)%len(p))) } }
	return m
}

func digest(g *gif.GIF) string {
	var b strings.Builder
	w, h := 0, 0
	for _, m := range g.Image { if m.Rect.Max.X > w { w = m.Rect.Max.X }; if m.Rect.Max.Y > h { h = m.Rect.Max.Y } }
	fmt.Fprintf(&b, "%dx%d/%d/%d/", w, h, g.LoopCount, len(g.Image))
	for i, m := range g.Image {
		fmt.Fprintf(&b, "%d,%d,%d,%d:%d:%d:", m.Bounds().Min.X, m.Bounds().Min.Y, m.Bounds().Dx(), m.Bounds().Dy(), g.Delay[i], g.Disposal[i])
		for y := m.Rect.Min.Y; y < m.Rect.Max.Y; y++ { for x := m.Rect.Min.X; x < m.Rect.Max.X; x++ { r, g, bl, a := m.At(x,y).RGBA(); fmt.Fprintf(&b, "%04x%04x%04x%04x", r,g,bl,a) } }
		b.WriteByte('/')
	}
	return b.String()
}
