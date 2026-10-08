package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"image/jpeg"
	"os"
	"strings"
)

// The oracle has two modes. "corpus" writes "label|hex" lines: streams this package encodes in every baseline, progressive,
// restart and sampling form, plus malformed variants of them. "decode" reads hex lines on stdin and prints "W,H|r,g,b,..." per
// image, or "fault". tools/ci/jpeg_check.tin runs both modes and the Tin decoder on the same lines.
func main() {
	mode := "corpus"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	switch mode {
	case "corpus":
		writeCorpus()
	case "decode":
		decodeLines()
	default:
		fmt.Fprintln(os.Stderr, "usage: jpeg [corpus|decode]")
		os.Exit(2)
	}
}

// decodeText decodes data with Go's image/jpeg and returns the "W,H|pixels" line, or "fault".
func decodeText(data []byte) string {
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return "fault"
	}
	b := img.Bounds()
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d,%d|", b.Dx(), b.Dy())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if x != b.Min.X || y != b.Min.Y {
				sb.WriteString(",")
			}
			fmt.Fprintf(&sb, "%d,%d,%d", r>>8, g>>8, bl>>8)
		}
	}
	return sb.String()
}

func decodeLines() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<28)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		data, err := hex.DecodeString(strings.TrimSpace(in.Text()))
		if err != nil {
			fmt.Fprintln(out, "fault")
			continue
		}
		fmt.Fprintln(out, decodeText(data))
	}
}

var (
	qualities = []int{1, 25, 50, 75, 90, 100}
	sizes     = [][2]int{{1, 1}, {9, 7}, {37, 29}}
	samplings = [][2]int{{1, 1}, {2, 1}, {1, 2}, {2, 2}}
)

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// writeCorpus encodes every configuration in every mode and restart interval, checks that the Go decoder reads each stream
// and that all of them give the same pixels (the coefficients are the same), then writes the lines.
func writeCorpus() {
	var lines []string
	valid := 0
	for _, gray := range []bool{true, false} {
		for _, q := range qualities {
			for _, sz := range sizes {
				for _, sm := range samplings {
					if gray && sm != samplings[0] {
						continue
					}
					var restarts []int
					if sz[0] == 37 {
						restarts = []int{0, 1, 2, 5}
					} else {
						restarts = []int{0}
					}
					base := config{gray: gray, quality: q, w: sz[0], h: sz[1], hy: sm[0], vy: sm[1]}
					want := ""
					for _, m := range []int{0, 1, 2} {
						for _, r := range restarts {
							if m != 0 && r > 0 && !gray && sm[0]*sm[1] > 1 {
								// Go counts the restart interval of a non-interleaved luma scan in padded MCUs of sm[0]*sm[1] blocks,
								// and rejects these streams; the spec counts blocks, and libjpeg decodes them like their restart-free twins.
								// The strict jpeg test checks the same shape against its baseline twin instead.
								continue
							}
							cfg := base
							cfg.mode = m
							cfg.restart = r
							e := encodeJPEG(cfg)
							got := decodeText(e.data)
							if got == "fault" {
								fail("Go rejects its own stream %s", label(cfg))
							}
							if want == "" {
								want = got
							} else if got != want {
								fail("Go decodes %s differently from the baseline stream", label(cfg))
							}
							lines = append(lines, label(cfg)+"|"+hex.EncodeToString(e.data))
							valid++
						}
					}
				}
			}
		}
	}
	malformed := malformedCases()
	lines = append(lines, malformed...)
	for _, l := range lines {
		fmt.Println(l)
	}
	fmt.Fprintf(os.Stderr, "jpeg corpus: %d valid streams, %d malformed\n", valid, len(malformed))
}

func label(c config) string {
	kind := "color"
	if c.gray {
		kind = "gray"
	}
	return fmt.Sprintf("go-%s-q%d-%dx%d-s%dx%d-m%d-r%d", kind, c.quality, c.w, c.h, c.hy, c.vy, c.mode, c.restart)
}

// malformedCases edits valid streams in ways both decoders must reject.
func malformedCases() []string {
	base := encodeJPEG(config{quality: 75, w: 37, h: 29, hy: 2, vy: 2, restart: 2, mode: 0})
	prog := encodeJPEG(config{quality: 75, w: 37, h: 29, hy: 2, vy: 1, restart: 0, mode: 2})
	var out []string
	add := func(name string, data []byte) {
		out = append(out, "malformed-"+name+"|"+hex.EncodeToString(data))
	}
	// lenient cases are rejected by the Tin decoder; Go's image/jpeg accepts them (it has no default table and returns a 0x0 image).
	lenient := func(name string, data []byte) {
		out = append(out, "lenient-malformed-"+name+"|"+hex.EncodeToString(data))
	}
	cut := func(d []byte, n int) []byte { return append([]byte(nil), d[:n]...) }
	edit := func(d []byte, at int, v ...byte) []byte {
		c := append([]byte(nil), d...)
		copy(c[at:], v)
		return c
	}
	remove := func(d []byte, at, n int) []byte {
		return append(append([]byte(nil), d[:at]...), d[at+n:]...)
	}
	add("signature", edit(base.data, 0, 0, 0))
	add("truncated-before-scan", cut(base.data, base.sos[0]))
	add("truncated-scan", cut(base.data, base.sos[0]+(len(base.data)-base.sos[0])/2))
	add("missing-eoi", cut(base.data, len(base.data)-2))
	add("missing-restart", remove(base.data, base.rst[0], 2))
	add("restart-out-of-order", edit(base.data, base.rst[0]+1, 0xd1))
	segLen := int(base.data[base.dqt+2])<<8 | int(base.data[base.dqt+3])
	lenient("dqt-missing", remove(base.data, base.dqt, segLen+2))
	add("scan-table-missing", edit(base.data, base.sos[0]+6, 0x33))
	add("scan-component-unknown", edit(base.data, base.sos[0]+5, 9))
	add("sampling-factor-5", edit(base.data, base.sof+11, 0x51))
	lenient("zero-width", edit(base.data, base.sof+7, 0, 0))
	add("progressive-dc-se-5", edit(prog.data, prog.sos[0]+12, 5))
	add("progressive-refine-al-mismatch", edit(prog.data, prog.sos[5]+9, 0x20))
	return out
}
