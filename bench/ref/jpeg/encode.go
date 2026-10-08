package main

import (
	"math"
	"math/bits"
)

// This encoder writes the corpus streams that Go's own encoder cannot: every luma sampling factor, restart intervals and
// progressive scans. Each table is flat (every symbol is 8 bits), so the streams are valid JPEG without optimization, and
// every configuration with the same coefficients is written in baseline and progressive form for the decoders to compare.

var zigzag = [64]int{
	0, 1, 8, 16, 9, 2, 3, 10, 17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 48, 41, 34, 27, 20, 13, 6, 7, 14, 21, 28,
	35, 42, 49, 56, 57, 50, 43, 36, 29, 22, 15, 23, 30, 37, 44, 51,
	58, 59, 52, 45, 38, 31, 39, 46, 53, 60, 61, 54, 47, 55, 62, 63,
}

// Base tables in zigzag order; the quality scales them as libjpeg does.
var stdLuma = [64]int{
	16, 11, 12, 14, 12, 10, 16, 14, 13, 14, 18, 17, 16, 19, 24, 40,
	26, 24, 22, 22, 24, 49, 35, 37, 29, 40, 58, 51, 61, 60, 57, 51,
	56, 55, 64, 72, 92, 78, 64, 68, 87, 69, 55, 56, 80, 109, 81, 87,
	95, 98, 103, 104, 103, 62, 77, 113, 121, 112, 100, 120, 92, 101, 103, 99,
}

var stdChroma = [64]int{
	17, 18, 18, 24, 21, 24, 47, 26, 26, 47, 99, 66, 56, 66, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99,
}

// configuration describes one corpus stream; mode 0 is baseline, 1 spectral selection, 2 successive approximation.
type config struct {
	gray    bool
	quality int
	w, h    int
	hy, vy  int
	restart int
	mode    int
}

// scanSpec is one scan: its components (indices into the frame), the spectral range and the approximation bits.
type scanSpec struct {
	comps  []int
	ss, se int
	ah, al int
}

// encoded is a stream with the byte offsets the malformed-input cases edit.
type encoded struct {
	data []byte
	dqt  int
	sof  int
	sos  []int
	rst  []int
}

type compInfo struct {
	h, v   int
	qt     int
	blocks []int
}

type blockRef struct{ c, idx int }

type huffTable struct {
	values []int
	index  map[int]int
}

func newTable(values []int) huffTable {
	t := huffTable{values: values, index: map[int]int{}}
	for i, v := range values {
		t.index[v] = i
	}
	return t
}

var dcTable = func() huffTable {
	var values []int
	for c := 0; c <= 11; c++ {
		values = append(values, c)
	}
	return newTable(values)
}()

// acTable holds EOB, EOBRUN 1..14, ZRL and every run/size pair with size 1 to 10.
var acTable = func() huffTable {
	var values []int
	for sym := 0; sym < 256; sym++ {
		run, size := sym>>4, sym&15
		if (size == 0 && run <= 14) || sym == 0xf0 || (size >= 1 && size <= 10) {
			values = append(values, sym)
		}
	}
	return newTable(values)
}()

type bitWriter struct {
	base int
	out  []byte
	acc  uint32
	n    uint
}

func (w *bitWriter) put(v int, n int) {
	for i := n - 1; i >= 0; i-- {
		w.acc = w.acc<<1 | uint32(v>>uint(i))&1
		w.n++
		if w.n == 8 {
			b := byte(w.acc)
			w.out = append(w.out, b)
			if b == 0xff {
				w.out = append(w.out, 0)
			}
			w.acc, w.n = 0, 0
		}
	}
}

func (w *bitWriter) pad() {
	for w.n != 0 {
		w.put(1, 1)
	}
}

func (w *bitWriter) symbol(t huffTable, sym int) {
	idx, ok := t.index[sym]
	if !ok {
		panic("jpeg encoder: symbol not in table")
	}
	w.put(idx, 8)
}

type jenc struct {
	w, h    int
	maxH    int
	maxV    int
	mcuCols int
	mcuRows int
	restart int
	comps   []compInfo
	pred    [3]int
	eobrun  int
	rstN    int
	rst     []int
}

func ceilDiv(a, b int) int { return (a + b - 1) / b }

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func extBits(v int, n int) int {
	if v < 0 {
		v--
	}
	return v & (1<<uint(n) - 1)
}

func quantTable(base [64]int, quality int) [64]int {
	scale := 200 - 2*quality
	if quality < 50 {
		scale = 5000 / quality
	}
	var q [64]int
	for k := 0; k < 64; k++ {
		v := (base[k]*scale + 50) / 100
		if v < 1 {
			v = 1
		}
		if v > 255 {
			v = 255
		}
		q[zigzag[k]] = v
	}
	return q
}

func (j *jenc) blockIndex(c, bx, by int) int {
	ci := j.comps[c]
	return ((by/ci.v)*j.mcuCols+bx/ci.h)*ci.h*ci.v + (by%ci.v)*ci.h + bx%ci.h
}

// units lists the blocks of each restart unit: one MCU of the interleaved components, or one block of a lone component.
func (j *jenc) units(comps []int) [][]blockRef {
	var units [][]blockRef
	if len(comps) == 1 {
		c := comps[0]
		bw := ceilDiv(ceilDiv(j.w*j.comps[c].h, j.maxH), 8)
		bh := ceilDiv(ceilDiv(j.h*j.comps[c].v, j.maxV), 8)
		for by := 0; by < bh; by++ {
			for bx := 0; bx < bw; bx++ {
				units = append(units, []blockRef{{c, j.blockIndex(c, bx, by)}})
			}
		}
		return units
	}
	for my := 0; my < j.mcuRows; my++ {
		for mx := 0; mx < j.mcuCols; mx++ {
			var u []blockRef
			for _, c := range comps {
				ci := j.comps[c]
				for by := 0; by < ci.v; by++ {
					for bx := 0; bx < ci.h; bx++ {
						u = append(u, blockRef{c, (my*j.mcuCols+mx)*ci.h*ci.v + by*ci.h + bx})
					}
				}
			}
			units = append(units, u)
		}
	}
	return units
}

func (j *jenc) flushEOBRun(w *bitWriter) {
	if j.eobrun == 0 {
		return
	}
	r := bits.Len(uint(j.eobrun)) - 1
	w.symbol(acTable, r<<4)
	w.put(j.eobrun-(1<<uint(r)), r)
	j.eobrun = 0
}

func (j *jenc) baselineBlock(w *bitWriter, c int, blk []int) {
	dc := blk[0]
	d := dc - j.pred[c]
	j.pred[c] = dc
	cat := bits.Len(uint(absInt(d)))
	w.symbol(dcTable, cat)
	if cat > 0 {
		w.put(extBits(d, cat), cat)
	}
	run := 0
	for k := 1; k < 64; k++ {
		v := blk[zigzag[k]]
		if v == 0 {
			run++
			continue
		}
		for run > 15 {
			w.symbol(acTable, 0xf0)
			run -= 16
		}
		cat := bits.Len(uint(absInt(v)))
		w.symbol(acTable, run<<4|cat)
		w.put(extBits(v, cat), cat)
		run = 0
	}
	if run > 0 {
		w.symbol(acTable, 0)
	}
}

func (j *jenc) dcFirst(w *bitWriter, c int, s scanSpec, blk []int) {
	v := blk[0] >> uint(s.al)
	d := v - j.pred[c]
	j.pred[c] = v
	cat := bits.Len(uint(absInt(d)))
	w.symbol(dcTable, cat)
	if cat > 0 {
		w.put(extBits(d, cat), cat)
	}
}

func (j *jenc) acFirst(w *bitWriter, s scanSpec, blk []int) {
	r := 0
	for k := s.ss; k <= s.se; k++ {
		v := blk[zigzag[k]]
		m := absInt(v) >> uint(s.al)
		if m == 0 {
			r++
			continue
		}
		if v < 0 {
			m = -m
		}
		j.flushEOBRun(w)
		for r > 15 {
			w.symbol(acTable, 0xf0)
			r -= 16
		}
		cat := bits.Len(uint(absInt(m)))
		w.symbol(acTable, r<<4|cat)
		w.put(extBits(m, cat), cat)
		r = 0
	}
	if r > 0 {
		j.eobrun++
		if j.eobrun == 0x7fff {
			j.flushEOBRun(w)
		}
	}
}

// acRefine follows libjpeg's refinement rules: history coefficients send one correction bit each, and every block ends with EOB.
func (j *jenc) acRefine(w *bitWriter, s scanSpec, blk []int) {
	mag := make([]int, 64)
	eob := -1
	for k := s.ss; k <= s.se; k++ {
		mag[k] = absInt(blk[zigzag[k]]) >> uint(s.al)
		if mag[k] == 1 {
			eob = k
		}
	}
	r := 0
	var br []int
	for k := s.ss; k <= s.se; k++ {
		m := mag[k]
		if m == 0 {
			r++
			continue
		}
		for r > 15 && k <= eob {
			w.symbol(acTable, 0xf0)
			r -= 16
			for _, b := range br {
				w.put(b, 1)
			}
			br = br[:0]
		}
		if m > 1 {
			br = append(br, m&1)
			continue
		}
		w.symbol(acTable, r<<4|1)
		sign := 0
		if blk[zigzag[k]] > 0 {
			sign = 1
		}
		w.put(sign, 1)
		for _, b := range br {
			w.put(b, 1)
		}
		br = br[:0]
		r = 0
	}
	if r > 0 || len(br) > 0 {
		w.symbol(acTable, 0)
		for _, b := range br {
			w.put(b, 1)
		}
	}
}

func (j *jenc) dcRefine(w *bitWriter, s scanSpec, blk []int) {
	w.put((blk[0]>>uint(s.al))&1, 1)
}

func (j *jenc) scan(w *bitWriter, progressive bool, s scanSpec) {
	j.pred = [3]int{}
	j.eobrun = 0
	j.rstN = 0
	for u, unit := range j.units(s.comps) {
		if j.restart > 0 && u > 0 && u%j.restart == 0 {
			j.flushEOBRun(w)
			w.pad()
			j.rst = append(j.rst, w.base+len(w.out))
			w.out = append(w.out, 0xff, byte(0xd0+j.rstN))
			j.rstN = (j.rstN + 1) % 8
			j.pred = [3]int{}
		}
		for _, b := range unit {
			blk := j.comps[b.c].blocks[b.idx*64 : b.idx*64+64]
			switch {
			case !progressive:
				j.baselineBlock(w, b.c, blk)
			case s.ss == 0 && s.ah == 0:
				j.dcFirst(w, b.c, s, blk)
			case s.ss == 0:
				j.dcRefine(w, s, blk)
			case s.ah == 0:
				j.acFirst(w, s, blk)
			default:
				j.acRefine(w, s, blk)
			}
		}
	}
	j.flushEOBRun(w)
	w.pad()
}

// componentSamples returns component c's samples, padded to the MCU grid; each averages the source pixels it covers.
func (j *jenc) componentSamples(src [][3]float64, c int, plane int) []float64 {
	ci := j.comps[c]
	fx, fy := j.maxH/ci.h, j.maxV/ci.v
	pw, ph := j.mcuCols*ci.h*8, j.mcuRows*ci.v*8
	out := make([]float64, pw*ph)
	for py := 0; py < ph; py++ {
		for px := 0; px < pw; px++ {
			sum := 0.0
			for dy := 0; dy < fy; dy++ {
				for dx := 0; dx < fx; dx++ {
					x := min(px*fx+dx, j.w-1)
					y := min(py*fy+dy, j.h-1)
					sum += src[y*j.w+x][plane]
				}
			}
			out[py*pw+px] = sum / float64(fx*fy)
		}
	}
	return out
}

func fdct(samples []float64, stride int, bx, by int, q *[64]int) []int {
	out := make([]int, 64)
	for v := 0; v < 8; v++ {
		for u := 0; u < 8; u++ {
			sum := 0.0
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					s := samples[(by*8+y)*stride+bx*8+x] - 128
					sum += s * math.Cos((2*float64(x)+1)*float64(u)*math.Pi/16) * math.Cos((2*float64(y)+1)*float64(v)*math.Pi/16)
				}
			}
			cu, cv := 1.0, 1.0
			if u == 0 {
				cu = math.Sqrt2 / 2
			}
			if v == 0 {
				cv = math.Sqrt2 / 2
			}
			f := 0.25 * cu * cv * sum / float64(q[v*8+u])
			out[v*8+u] = int(math.Round(f))
		}
	}
	return out
}

// encodeJPEG writes one stream for cfg; the same coefficients come out whatever the mode and restart interval.
func encodeJPEG(cfg config) encoded {
	var comps []compInfo
	if cfg.gray {
		comps = []compInfo{{h: 1, v: 1, qt: 0}}
	} else {
		comps = []compInfo{{h: cfg.hy, v: cfg.vy, qt: 0}, {h: 1, v: 1, qt: 1}, {h: 1, v: 1, qt: 1}}
	}
	j := &jenc{w: cfg.w, h: cfg.h, maxH: 1, maxV: 1, restart: cfg.restart, comps: comps}
	for _, c := range comps {
		j.maxH = max(j.maxH, c.h)
		j.maxV = max(j.maxV, c.v)
	}
	j.mcuCols = ceilDiv(cfg.w, 8*j.maxH)
	j.mcuRows = ceilDiv(cfg.h, 8*j.maxV)
	src := make([][3]float64, cfg.w*cfg.h)
	for y := 0; y < cfg.h; y++ {
		for x := 0; x < cfg.w; x++ {
			r, g, b := sourcePixel(x, y)
			if cfg.gray {
				src[y*cfg.w+x] = [3]float64{float64(r), 0, 0}
				continue
			}
			src[y*cfg.w+x] = [3]float64{
				0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b),
				-0.168736*float64(r) - 0.331264*float64(g) + 0.5*float64(b) + 128,
				0.5*float64(r) - 0.418688*float64(g) - 0.081312*float64(b) + 128,
			}
		}
	}
	quants := [2][64]int{quantTable(stdLuma, cfg.quality), quantTable(stdChroma, cfg.quality)}
	for c := range j.comps {
		ci := &j.comps[c]
		plane := 0
		if c > 0 {
			plane = c
		}
		samples := j.componentSamples(src, c, plane)
		stride := j.mcuCols * ci.h * 8
		ci.blocks = make([]int, j.mcuCols*j.mcuRows*ci.h*ci.v*64)
		q := &quants[ci.qt]
		for by := 0; by < j.mcuRows*ci.v; by++ {
			for bx := 0; bx < j.mcuCols*ci.h; bx++ {
				coef := fdct(samples, stride, bx, by, q)
				copy(ci.blocks[j.blockIndex(c, bx, by)*64:], coef)
			}
		}
	}
	return j.write(cfg, quants)
}

// sourcePixel is the deterministic test image shared by every corpus configuration.
func sourcePixel(x, y int) (int, int, int) {
	return (x*37 + y*13 + 7) % 256, (x*11 + y*47 + 19) % 256, (x*23 + y*29 + 31) % 256
}

func segment(out []byte, marker byte, payload []byte) []byte {
	n := len(payload) + 2
	out = append(out, 0xff, marker, byte(n>>8), byte(n))
	return append(out, payload...)
}

func (j *jenc) write(cfg config, quants [2][64]int) encoded {
	var e encoded
	out := []byte{0xff, 0xd8}
	e.dqt = len(out)
	var dqt []byte
	for t := 0; t < 2; t++ {
		if t == 1 && cfg.gray {
			break
		}
		dqt = append(dqt, byte(t))
		for k := 0; k < 64; k++ {
			dqt = append(dqt, byte(quants[t][zigzag[k]]))
		}
	}
	out = segment(out, 0xdb, dqt)
	e.sof = len(out)
	sof := []byte{8, byte(cfg.h >> 8), byte(cfg.h), byte(cfg.w >> 8), byte(cfg.w), byte(len(j.comps))}
	for i, c := range j.comps {
		sof = append(sof, byte(i+1), byte(c.h<<4|c.v), byte(c.qt))
	}
	sofMarker := byte(0xc0)
	if cfg.mode != 0 {
		sofMarker = 0xc2
	}
	out = segment(out, sofMarker, sof)
	dht := []byte{0x00}
	dht = appendCounts(dht, len(dcTable.values))
	for _, v := range dcTable.values {
		dht = append(dht, byte(v))
	}
	dht = append(dht, 0x10)
	dht = appendCounts(dht, len(acTable.values))
	for _, v := range acTable.values {
		dht = append(dht, byte(v))
	}
	out = segment(out, 0xc4, dht)
	if cfg.restart > 0 {
		out = segment(out, 0xdd, []byte{byte(cfg.restart >> 8), byte(cfg.restart)})
	}
	all := make([]int, len(j.comps))
	for i := range all {
		all[i] = i
	}
	scans := []scanSpec{{comps: all, ss: 0, se: 63}}
	if cfg.mode == 1 {
		scans = spectralScript(cfg.gray)
	} else if cfg.mode == 2 {
		scans = successiveScript(cfg.gray)
	}
	for _, s := range scans {
		e.sos = append(e.sos, len(out))
		sos := []byte{byte(len(s.comps))}
		for _, c := range s.comps {
			sos = append(sos, byte(c+1), 0)
		}
		sos = append(sos, byte(s.ss), byte(s.se), byte(s.ah<<4|s.al))
		out = segment(out, 0xda, sos)
		w := &bitWriter{base: len(out)}
		j.scan(w, cfg.mode != 0, s)
		e.rst = append(e.rst, j.rst...)
		j.rst = nil
		out = append(out, w.out...)
	}
	out = append(out, 0xff, 0xd9)
	e.data = out
	return e
}

func appendCounts(dht []byte, n int) []byte {
	counts := make([]byte, 16)
	counts[7] = byte(n)
	return append(dht, counts...)
}

func spectralScript(gray bool) []scanSpec {
	if gray {
		return []scanSpec{{comps: []int{0}, ss: 0, se: 0}, {comps: []int{0}, ss: 1, se: 63}}
	}
	return []scanSpec{
		{comps: []int{0, 1, 2}, ss: 0, se: 0},
		{comps: []int{0}, ss: 1, se: 63},
		{comps: []int{1}, ss: 1, se: 63},
		{comps: []int{2}, ss: 1, se: 63},
	}
}

// successiveScript sends each band's first scan at a coarse shift and refines it down to bit 0.
func successiveScript(gray bool) []scanSpec {
	if gray {
		return []scanSpec{
			{comps: []int{0}, ss: 0, se: 0, al: 1},
			{comps: []int{0}, ss: 1, se: 5, al: 2},
			{comps: []int{0}, ss: 6, se: 63, al: 2},
			{comps: []int{0}, ss: 1, se: 5, ah: 2, al: 1},
			{comps: []int{0}, ss: 1, se: 5, ah: 1, al: 0},
			{comps: []int{0}, ss: 6, se: 63, ah: 2, al: 1},
			{comps: []int{0}, ss: 6, se: 63, ah: 1, al: 0},
			{comps: []int{0}, ss: 0, se: 0, ah: 1, al: 0},
		}
	}
	return []scanSpec{
		{comps: []int{0, 1, 2}, ss: 0, se: 0, al: 1},
		{comps: []int{0}, ss: 1, se: 5, al: 2},
		{comps: []int{1}, ss: 1, se: 63, al: 1},
		{comps: []int{2}, ss: 1, se: 63, al: 1},
		{comps: []int{0}, ss: 6, se: 63, al: 2},
		{comps: []int{0}, ss: 1, se: 5, ah: 2, al: 1},
		{comps: []int{0}, ss: 1, se: 5, ah: 1, al: 0},
		{comps: []int{0}, ss: 6, se: 63, ah: 2, al: 1},
		{comps: []int{0}, ss: 6, se: 63, ah: 1, al: 0},
		{comps: []int{1}, ss: 1, se: 63, ah: 1, al: 0},
		{comps: []int{2}, ss: 1, se: 63, ah: 1, al: 0},
		{comps: []int{0, 1, 2}, ss: 0, se: 0, ah: 1, al: 0},
	}
}
