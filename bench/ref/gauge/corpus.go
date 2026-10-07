package main

// The corpus check: every transcendental function over the same deterministic inputs as
// bench/ref/gauge/corpus.tin, printing a hash of the result bits per function (and every result
// with "dump"). Equal hashes mean equal to the last bit. Exp is Go's pure algorithm (copied below),
// because on arm64 and amd64 math.Exp is assembly with a different last bit.

import (
	"fmt"
	"math"
	"os"
	"strconv"
)

type rng struct{ s uint64 }

func (r *rng) next() uint64 {
	r.s ^= r.s >> 12
	r.s ^= r.s << 25
	r.s ^= r.s >> 27
	return r.s * 2685821657736338717
}

var edge = []float64{
	0, math.Copysign(0, -1), 1, -1, 0.5, -0.5, 2, -2, 3, 10, 100, 1e-3, 1e-10, 1e-300, 1e300, -1e300,
	math.Inf(1), math.Inf(-1), math.NaN(), math.MaxFloat64, -math.MaxFloat64, math.SmallestNonzeroFloat64,
	-math.SmallestNonzeroFloat64, 2.2250738585072014e-308, 2.225073858507201e-308, 1e-310, 1e-320,
	0.625, 0.66, 0.7, 21, 21.000000000000004, 44.01, 88.03, 709.78, 709.782712893384, 710, -745.13, -745.2,
	1023.9999999999999, 1024, -1074, -1075, 3.725290298461914e-09, 1.862645149230957e-09, 5.551115123125783e-17,
	0.4142135623730951, -0.2928932188134525, 1.4142135623730951, 0.7071067811865476, 0.7853981633974483,
	1.5707963267948966, 3.141592653589793, 6.283185307179586, 536870911, 536870912, 536870913, 1073741824,
	1e10, 1e15, 4503599627370496, 9007199254740992, 9007199254740993, 1e22, 1e100, 1.7e308,
	9.223372036854776e18, 1.8446744073709552e19, 27, -27, 8, 1000, 0.001, 0.1, 0.2, 0.3, 123.456,
}

// count is the number of random inputs (GAUGE_N, default 20000; the CI twin uses 120000).
func count() int {
	if s := os.Getenv("GAUGE_N"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			return v
		}
	}
	return 20000
}

func inputs() []float64 {
	xs := append([]float64{}, edge...)
	r := &rng{s: 88172645463325252}
	for i := 0; i < count(); i++ {
		v := r.next()
		m := r.next()
		switch v % 4 {
		case 0:
			xs = append(xs, float64(int(m>>11)%2000001-1000000)/100000)
		case 1:
			e := int(v>>8) % 121
			if e < 0 {
				e = -e
			}
			e -= 60
			bits := uint64(1023+e)<<52 | m&(1<<52-1) | (v>>20&1)<<63
			xs = append(xs, math.Float64frombits(bits))
		case 2:
			xs = append(xs, math.Float64frombits(m))
		default:
			k := int(v>>8)%8001 - 4000
			xs = append(xs, float64(k)/8)
		}
	}
	return xs
}

// Go's exp(), without the assembly fast path.
func pureExp(x float64) float64 {
	const (
		Ln2Hi     = 6.93147180369123816490e-01
		Ln2Lo     = 1.90821492927058770002e-10
		Log2e     = 1.44269504088896338700e+00
		Overflow  = 7.09782712893383973096e+02
		Underflow = -7.45133219101941108420e+02
		NearZero  = 1.0 / (1 << 28)
	)
	switch {
	case math.IsNaN(x):
		return x
	case x > Overflow:
		return math.Inf(1)
	case x < Underflow:
		return 0
	case -NearZero < x && x < NearZero:
		return 1 + x
	}
	var k int
	switch {
	case x < 0:
		k = int(Log2e*x - 0.5)
	case x > 0:
		k = int(Log2e*x + 0.5)
	}
	hi := x - float64(k)*Ln2Hi
	lo := float64(k) * Ln2Lo
	return expmulti(hi, lo, k)
}

func expmulti(hi, lo float64, k int) float64 {
	const (
		P1 = 1.66666666666666657415e-01
		P2 = -2.77777777770155933842e-03
		P3 = 6.61375632143793436117e-05
		P4 = -1.65339022054652515390e-06
		P5 = 4.13813679705723846039e-08
	)
	r := hi - lo
	t := r * r
	c := r - t*(P1+t*(P2+t*(P3+t*(P4+t*P5))))
	y := 1 - ((lo - (r*c)/(2-c)) - hi)
	return math.Ldexp(y, k)
}

func pureExp2(x float64) float64 {
	const (
		Ln2Hi     = 6.93147180369123816490e-01
		Ln2Lo     = 1.90821492927058770002e-10
		Overflow  = 1.0239999999999999e+03
		Underflow = -1.0740e+03
	)
	switch {
	case math.IsNaN(x):
		return x
	case x > Overflow:
		return math.Inf(1)
	case x < Underflow:
		return 0
	}
	var k int
	switch {
	case x > 0:
		k = int(x + 0.5)
	case x < 0:
		k = int(x - 0.5)
	}
	t := x - float64(k)
	hi := t * Ln2Hi
	lo := -t * Ln2Lo
	return expmulti(hi, lo, k)
}

// pureSinh and pureCosh are Go's sinh and cosh over pureExp.
func pureSinh(x float64) float64 {
	const (
		P0 = -0.6307673640497716991184787251e+6
		P1 = -0.8991272022039509355398013511e+5
		P2 = -0.2894211355989563807284660366e+4
		P3 = -0.2630563213397497062819489e+2
		Q0 = -0.6307673640497716991212077277e+6
		Q1 = 0.1521517378790019070696485176e+5
		Q2 = -0.173678953558233699533450911e+3
	)
	sign := false
	if x < 0 {
		x = -x
		sign = true
	}
	var temp float64
	switch {
	case x > 21:
		temp = pureExp(x) * 0.5
	case x > 0.5:
		ex := pureExp(x)
		temp = (ex - 1/ex) * 0.5
	default:
		sq := x * x
		temp = (((P3*sq+P2)*sq+P1)*sq + P0) * x
		temp = temp / (((sq+Q2)*sq+Q1)*sq + Q0)
	}
	if sign {
		temp = -temp
	}
	return temp
}

func pureCosh(x float64) float64 {
	x = math.Abs(x)
	if x > 21 {
		return pureExp(x) * 0.5
	}
	ex := pureExp(x)
	return (ex + 1/ex) * 0.5
}

var tanhP = [...]float64{-9.64399179425052238628e-1, -9.92877231001918586564e1, -1.61468768441708447952e3}
var tanhQ = [...]float64{1.12811678491632931402e2, 2.23548839060100448583e3, 4.84406305325125486048e3}

func pureTanh(x float64) float64 {
	const MAXLOG = 8.8029691931113054295988e+01
	z := math.Abs(x)
	switch {
	case z > 0.5*MAXLOG:
		if x < 0 {
			return -1
		}
		return 1
	case z >= 0.625:
		s := pureExp(2 * z)
		z = 1 - 2/(s+1)
		if x < 0 {
			z = -z
		}
	default:
		if x == 0 {
			return x
		}
		s := x * x
		z = x + x*s*((tanhP[0]*s+tanhP[1])*s+tanhP[2])/(((s+tanhQ[0])*s+tanhQ[1])*s+tanhQ[2])
	}
	return z
}

// pureLog2 and pureLog10 are the same as math's: neither uses Exp.
// pow uses Exp for fractional exponents.
func purePow(x, y float64) float64 {
	// math.Pow calls math.Exp (assembly on arm64/amd64) for the fractional part, so replace that
	// call: copy of math.pow with pureExp.
	isOddInt := func(x float64) bool {
		if math.Abs(x) >= (1 << 53) {
			return false
		}
		xi, xf := math.Modf(x)
		return xf == 0 && int64(xi)&1 == 1
	}
	switch {
	case y == 0 || x == 1:
		return 1
	case y == 1:
		return x
	case math.IsNaN(x) || math.IsNaN(y):
		return math.NaN()
	case x == 0:
		switch {
		case y < 0:
			if math.Signbit(x) && isOddInt(y) {
				return math.Inf(-1)
			}
			return math.Inf(1)
		case y > 0:
			if math.Signbit(x) && isOddInt(y) {
				return x
			}
			return 0
		}
	case math.IsInf(y, 0):
		switch {
		case x == -1:
			return 1
		case (math.Abs(x) < 1) == math.IsInf(y, 1):
			return 0
		default:
			return math.Inf(1)
		}
	case math.IsInf(x, 0):
		if math.IsInf(x, -1) {
			return purePow(1/x, -y)
		}
		switch {
		case y < 0:
			return 0
		case y > 0:
			return math.Inf(1)
		}
	case y == 0.5:
		return math.Sqrt(x)
	case y == -0.5:
		return 1 / math.Sqrt(x)
	}
	yi, yf := math.Modf(math.Abs(y))
	if yf != 0 && x < 0 {
		return math.NaN()
	}
	if yi >= 1<<63 {
		switch {
		case x == -1:
			return 1
		case (math.Abs(x) < 1) == (y > 0):
			return 0
		default:
			return math.Inf(1)
		}
	}
	a1 := 1.0
	ae := 0
	if yf != 0 {
		if yf > 0.5 {
			yf--
			yi++
		}
		a1 = pureExp(yf * math.Log(x))
	}
	x1, xe := math.Frexp(x)
	for i := int64(yi); i != 0; i >>= 1 {
		if xe < -1<<12 || 1<<12 < xe {
			ae += xe
			break
		}
		if i&1 == 1 {
			a1 *= x1
			ae += xe
		}
		x1 *= x1
		xe <<= 1
		if x1 < .5 {
			x1 += x1
			xe--
		}
	}
	if y < 0 {
		a1 = 1 / a1
		ae = -ae
	}
	return math.Ldexp(a1, ae)
}

type unary struct {
	name string
	f    func(float64) float64
}

type binary struct {
	name string
	f    func(float64, float64) float64
}

var unaries = []unary{
	{"sin", math.Sin}, {"cos", math.Cos}, {"tan", math.Tan}, {"asin", math.Asin}, {"acos", math.Acos},
	{"atan", math.Atan}, {"sinh", pureSinh}, {"cosh", pureCosh}, {"tanh", pureTanh}, {"exp", pureExp},
	{"exp2", pureExp2}, {"log", math.Log}, {"log2", math.Log2}, {"log10", math.Log10}, {"log1p", math.Log1p},
	{"cbrt", math.Cbrt},
	{"expm1", math.Expm1}, {"asinh", math.Asinh}, {"acosh", math.Acosh}, {"atanh", math.Atanh},
	{"logb", math.Logb}, {"f32frombits", f32frombitsf}, {"erf", pureErf}, {"erfc", pureErfc},
	{"erfinv", math.Erfinv}, {"erfcinv", math.Erfcinv}, {"gamma", pureGamma},
}

var binaries = []binary{
	{"atan2", math.Atan2}, {"pow", purePow}, {"hypot", math.Hypot}, {"mod", math.Mod},
	{"remainder", math.Remainder}, {"dim", math.Dim}, {"nextafter", math.Nextafter},
}

func canon(f float64) uint64 {
	if f != f {
		return 0x7ff8000000000001
	}
	return math.Float64bits(f)
}

type hasher struct{ h uint64 }

func (h *hasher) add(b uint64) {
	for i := 0; i < 8; i++ {
		h.h ^= b >> (8 * uint(i)) & 0xff
		h.h *= 1099511628211
	}
}

// The error function, the complementary error function and the gamma function: Go's erf.go and
// gamma.go with pureExp and purePow, because math.Erf, math.Erfc and math.Gamma call the
// assembly Exp and Pow on arm64 and amd64 (math.Erfinv, math.Erfcinv and math.Lgamma are pure).

const (
	erx = 8.45062911510467529297e-01 // 0x3FEB0AC160000000
	// Coefficients for approximation to  erf in [0, 0.84375]
	efx  = 1.28379167095512586316e-01  // 0x3FC06EBA8214DB69
	efx8 = 1.02703333676410069053e+00  // 0x3FF06EBA8214DB69
	pp0  = 1.28379167095512558561e-01  // 0x3FC06EBA8214DB68
	pp1  = -3.25042107247001499370e-01 // 0xBFD4CD7D691CB913
	pp2  = -2.84817495755985104766e-02 // 0xBF9D2A51DBD7194F
	pp3  = -5.77027029648944159157e-03 // 0xBF77A291236668E4
	pp4  = -2.37630166566501626084e-05 // 0xBEF8EAD6120016AC
	qq1  = 3.97917223959155352819e-01  // 0x3FD97779CDDADC09
	qq2  = 6.50222499887672944485e-02  // 0x3FB0A54C5536CEBA
	qq3  = 5.08130628187576562776e-03  // 0x3F74D022C4D36B0F
	qq4  = 1.32494738004321644526e-04  // 0x3F215DC9221C1A10
	qq5  = -3.96022827877536812320e-06 // 0xBED09C4342A26120
	// Coefficients for approximation to  erf  in [0.84375, 1.25]
	pa0 = -2.36211856075265944077e-03 // 0xBF6359B8BEF77538
	pa1 = 4.14856118683748331666e-01  // 0x3FDA8D00AD92B34D
	pa2 = -3.72207876035701323847e-01 // 0xBFD7D240FBB8C3F1
	pa3 = 3.18346619901161753674e-01  // 0x3FD45FCA805120E4
	pa4 = -1.10894694282396677476e-01 // 0xBFBC63983D3E28EC
	pa5 = 3.54783043256182359371e-02  // 0x3FA22A36599795EB
	pa6 = -2.16637559486879084300e-03 // 0xBF61BF380A96073F
	qa1 = 1.06420880400844228286e-01  // 0x3FBB3E6618EEE323
	qa2 = 5.40397917702171048937e-01  // 0x3FE14AF092EB6F33
	qa3 = 7.18286544141962662868e-02  // 0x3FB2635CD99FE9A7
	qa4 = 1.26171219808761642112e-01  // 0x3FC02660E763351F
	qa5 = 1.36370839120290507362e-02  // 0x3F8BEDC26B51DD1C
	qa6 = 1.19844998467991074170e-02  // 0x3F888B545735151D
	// Coefficients for approximation to  erfc in [1.25, 1/0.35]
	ra0 = -9.86494403484714822705e-03 // 0xBF843412600D6435
	ra1 = -6.93858572707181764372e-01 // 0xBFE63416E4BA7360
	ra2 = -1.05586262253232909814e+01 // 0xC0251E0441B0E726
	ra3 = -6.23753324503260060396e+01 // 0xC04F300AE4CBA38D
	ra4 = -1.62396669462573470355e+02 // 0xC0644CB184282266
	ra5 = -1.84605092906711035994e+02 // 0xC067135CEBCCABB2
	ra6 = -8.12874355063065934246e+01 // 0xC054526557E4D2F2
	ra7 = -9.81432934416914548592e+00 // 0xC023A0EFC69AC25C
	sa1 = 1.96512716674392571292e+01  // 0x4033A6B9BD707687
	sa2 = 1.37657754143519042600e+02  // 0x4061350C526AE721
	sa3 = 4.34565877475229228821e+02  // 0x407B290DD58A1A71
	sa4 = 6.45387271733267880336e+02  // 0x40842B1921EC2868
	sa5 = 4.29008140027567833386e+02  // 0x407AD02157700314
	sa6 = 1.08635005541779435134e+02  // 0x405B28A3EE48AE2C
	sa7 = 6.57024977031928170135e+00  // 0x401A47EF8E484A93
	sa8 = -6.04244152148580987438e-02 // 0xBFAEEFF2EE749A62
	// Coefficients for approximation to  erfc in [1/.35, 28]
	rb0 = -9.86494292470009928597e-03 // 0xBF84341239E86F4A
	rb1 = -7.99283237680523006574e-01 // 0xBFE993BA70C285DE
	rb2 = -1.77579549177547519889e+01 // 0xC031C209555F995A
	rb3 = -1.60636384855821916062e+02 // 0xC064145D43C5ED98
	rb4 = -6.37566443368389627722e+02 // 0xC083EC881375F228
	rb5 = -1.02509513161107724954e+03 // 0xC09004616A2E5992
	rb6 = -4.83519191608651397019e+02 // 0xC07E384E9BDC383F
	sb1 = 3.03380607434824582924e+01  // 0x403E568B261D5190
	sb2 = 3.25792512996573918826e+02  // 0x40745CAE221B9F0A
	sb3 = 1.53672958608443695994e+03  // 0x409802EB189D5118
	sb4 = 3.19985821950859553908e+03  // 0x40A8FFB7688C246A
	sb5 = 2.55305040643316442583e+03  // 0x40A3F219CEDF3BE6
	sb6 = 4.74528541206955367215e+02  // 0x407DA874E79FE763
	sb7 = -2.24409524465858183362e+01 // 0xC03670E242712D62
)

func pureErf(x float64) float64 {
	const (
		VeryTiny = 2.848094538889218e-306 // 0x0080000000000000
		Small    = 1.0 / (1 << 28)        // 2**-28
	)
	// special cases
	switch {
	case math.IsNaN(x):
		return math.NaN()
	case math.IsInf(x, 1):
		return 1
	case math.IsInf(x, -1):
		return -1
	}
	sign := false
	if x < 0 {
		x = -x
		sign = true
	}
	if x < 0.84375 { // |x| < 0.84375
		var temp float64
		if x < Small { // |x| < 2**-28
			if x < VeryTiny {
				temp = 0.125 * (8.0*x + efx8*x) // avoid underflow
			} else {
				temp = x + efx*x
			}
		} else {
			z := x * x
			r := pp0 + z*(pp1+z*(pp2+z*(pp3+z*pp4)))
			s := 1 + z*(qq1+z*(qq2+z*(qq3+z*(qq4+z*qq5))))
			y := r / s
			temp = x + x*y
		}
		if sign {
			return -temp
		}
		return temp
	}
	if x < 1.25 { // 0.84375 <= |x| < 1.25
		s := x - 1
		P := pa0 + s*(pa1+s*(pa2+s*(pa3+s*(pa4+s*(pa5+s*pa6)))))
		Q := 1 + s*(qa1+s*(qa2+s*(qa3+s*(qa4+s*(qa5+s*qa6)))))
		if sign {
			return -erx - P/Q
		}
		return erx + P/Q
	}
	if x >= 6 { // inf > |x| >= 6
		if sign {
			return -1
		}
		return 1
	}
	s := 1 / (x * x)
	var R, S float64
	if x < 1/0.35 { // |x| < 1 / 0.35  ~ 2.857143
		R = ra0 + s*(ra1+s*(ra2+s*(ra3+s*(ra4+s*(ra5+s*(ra6+s*ra7))))))
		S = 1 + s*(sa1+s*(sa2+s*(sa3+s*(sa4+s*(sa5+s*(sa6+s*(sa7+s*sa8)))))))
	} else { // |x| >= 1 / 0.35  ~ 2.857143
		R = rb0 + s*(rb1+s*(rb2+s*(rb3+s*(rb4+s*(rb5+s*rb6)))))
		S = 1 + s*(sb1+s*(sb2+s*(sb3+s*(sb4+s*(sb5+s*(sb6+s*sb7))))))
	}
	z := math.Float64frombits(math.Float64bits(x) & 0xffffffff00000000) // pseudo-single (20-bit) precision x
	r := pureExp(-z*z-0.5625) * pureExp((z-x)*(z+x)+R/S)
	if sign {
		return r/x - 1
	}
	return 1 - r/x
}

// Erfc returns the complementary error function of x.
//
// Special cases are:
//
//	Erfc(+math.Inf) = 0
//	Erfc(-math.Inf) = 2
//	Erfc(math.NaN) = math.NaN
func pureErfc(x float64) float64 {
	const Tiny = 1.0 / (1 << 56) // 2**-56
	// special cases
	switch {
	case math.IsNaN(x):
		return math.NaN()
	case math.IsInf(x, 1):
		return 0
	case math.IsInf(x, -1):
		return 2
	}
	sign := false
	if x < 0 {
		x = -x
		sign = true
	}
	if x < 0.84375 { // |x| < 0.84375
		var temp float64
		if x < Tiny { // |x| < 2**-56
			temp = x
		} else {
			z := x * x
			r := pp0 + z*(pp1+z*(pp2+z*(pp3+z*pp4)))
			s := 1 + z*(qq1+z*(qq2+z*(qq3+z*(qq4+z*qq5))))
			y := r / s
			if x < 0.25 { // |x| < 1/4
				temp = x + x*y
			} else {
				temp = 0.5 + (x*y + (x - 0.5))
			}
		}
		if sign {
			return 1 + temp
		}
		return 1 - temp
	}
	if x < 1.25 { // 0.84375 <= |x| < 1.25
		s := x - 1
		P := pa0 + s*(pa1+s*(pa2+s*(pa3+s*(pa4+s*(pa5+s*pa6)))))
		Q := 1 + s*(qa1+s*(qa2+s*(qa3+s*(qa4+s*(qa5+s*qa6)))))
		if sign {
			return 1 + erx + P/Q
		}
		return 1 - erx - P/Q

	}
	if x < 28 { // |x| < 28
		s := 1 / (x * x)
		var R, S float64
		if x < 1/0.35 { // |x| < 1 / 0.35 ~ 2.857143
			R = ra0 + s*(ra1+s*(ra2+s*(ra3+s*(ra4+s*(ra5+s*(ra6+s*ra7))))))
			S = 1 + s*(sa1+s*(sa2+s*(sa3+s*(sa4+s*(sa5+s*(sa6+s*(sa7+s*sa8)))))))
		} else { // |x| >= 1 / 0.35 ~ 2.857143
			if sign && x > 6 {
				return 2 // x < -6
			}
			R = rb0 + s*(rb1+s*(rb2+s*(rb3+s*(rb4+s*(rb5+s*rb6)))))
			S = 1 + s*(sb1+s*(sb2+s*(sb3+s*(sb4+s*(sb5+s*(sb6+s*sb7))))))
		}
		z := math.Float64frombits(math.Float64bits(x) & 0xffffffff00000000) // pseudo-single (20-bit) precision x
		r := pureExp(-z*z-0.5625) * pureExp((z-x)*(z+x)+R/S)
		if sign {
			return 2 - r/x
		}
		return r / x
	}
	if sign {
		return 2
	}
	return 0
}

var _gamP = [...]float64{
	1.60119522476751861407e-04,
	1.19135147006586384913e-03,
	1.04213797561761569935e-02,
	4.76367800457137231464e-02,
	2.07448227648435975150e-01,
	4.94214826801497100753e-01,
	9.99999999999999996796e-01,
}
var _gamQ = [...]float64{
	-2.31581873324120129819e-05,
	5.39605580493303397842e-04,
	-4.45641913851797240494e-03,
	1.18139785222060435552e-02,
	3.58236398605498653373e-02,
	-2.34591795718243348568e-01,
	7.14304917030273074085e-02,
	1.00000000000000000320e+00,
}
var _gamS = [...]float64{
	7.87311395793093628397e-04,
	-2.29549961613378126380e-04,
	-2.68132617805781232825e-03,
	3.47222221605458667310e-03,
	8.33333333333482257126e-02,
}

// Gamma function computed by Stirling's formula.
// The pair of results must be multiplied together to get the actual answer.
// The multiplication is left to the caller so that, if careful, the caller can avoid
// infinity for 172 <= x <= 180.
// The polynomial is valid for 33 <= x <= 172; larger values are only used
// in reciprocal and produce denormalized floats. The lower precision there
// masks any imprecision in the polynomial.
func pureStirling(x float64) (float64, float64) {
	if x > 200 {
		return math.Inf(1), 1
	}
	const (
		SqrtTwoPi   = 2.506628274631000502417
		MaxStirling = 143.01608
	)
	w := 1 / x
	w = 1 + w*((((_gamS[0]*w+_gamS[1])*w+_gamS[2])*w+_gamS[3])*w+_gamS[4])
	y1 := pureExp(x)
	y2 := 1.0
	if x > MaxStirling { // avoid purePow() overflow
		v := purePow(x, 0.5*x-0.25)
		y1, y2 = v, v/y1
	} else {
		y1 = purePow(x, x-0.5) / y1
	}
	return y1, SqrtTwoPi * w * y2
}

// Gamma returns the Gamma function of x.
//
// Special cases are:
//
//	Gamma(+math.Inf) = +math.Inf
//	Gamma(+0) = +math.Inf
//	Gamma(-0) = -math.Inf
//	Gamma(x) = math.NaN for integer x < 0
//	Gamma(-math.Inf) = math.NaN
//	Gamma(math.NaN) = math.NaN
func pureGamma(x float64) float64 {
	const Euler = 0.57721566490153286060651209008240243104215933593992 // A001620
	// special cases
	switch {
	case isNegInt(x) || math.IsInf(x, -1) || math.IsNaN(x):
		return math.NaN()
	case math.IsInf(x, 1):
		return math.Inf(1)
	case x == 0:
		if math.Signbit(x) {
			return math.Inf(-1)
		}
		return math.Inf(1)
	}
	q := math.Abs(x)
	p := math.Floor(q)
	if q > 33 {
		if x >= 0 {
			y1, y2 := pureStirling(x)
			return y1 * y2
		}
		// Note: x is negative but (checked above) not a negative integer,
		// so x must be small enough to be in range for conversion to int64.
		// If |x| were >= 2⁶³ it would have to be an integer.
		signgam := 1
		if ip := int64(p); ip&1 == 0 {
			signgam = -1
		}
		z := q - p
		if z > 0.5 {
			p = p + 1
			z = q - p
		}
		z = q * math.Sin(math.Pi*z)
		if z == 0 {
			return math.Inf(signgam)
		}
		sq1, sq2 := pureStirling(q)
		absz := math.Abs(z)
		d := absz * sq1 * sq2
		if math.IsInf(d, 0) {
			z = math.Pi / absz / sq1 / sq2
		} else {
			z = math.Pi / d
		}
		return float64(signgam) * z
	}

	// Reduce argument
	z := 1.0
	for x >= 3 {
		x = x - 1
		z = z * x
	}
	for x < 0 {
		if x > -1e-09 {
			goto small
		}
		z = z / x
		x = x + 1
	}
	for x < 2 {
		if x < 1e-09 {
			goto small
		}
		z = z / x
		x = x + 1
	}

	if x == 2 {
		return z
	}

	x = x - 2
	p = (((((x*_gamP[0]+_gamP[1])*x+_gamP[2])*x+_gamP[3])*x+_gamP[4])*x+_gamP[5])*x + _gamP[6]
	q = ((((((x*_gamQ[0]+_gamQ[1])*x+_gamQ[2])*x+_gamQ[3])*x+_gamQ[4])*x+_gamQ[5])*x+_gamQ[6])*x + _gamQ[7]
	return z * p / q

small:
	if x == 0 {
		return math.Inf(1)
	}
	return z / ((1 + Euler*x) * x)
}

func isNegInt(x float64) bool {
	return x < 0 && x == math.Floor(x)
}

// unarylg hashes a float result and an integer sign per input, as Lgamma returns.
func unarylg(name string, f func(float64) (float64, int), xs []float64, dump bool) {
	if wantedSkip(name, dump) {
		return
	}
	h := hasher{14695981039346656037}
	for i, x := range xs {
		lg, sign := f(x)
		b := canon(lg)
		h.add(b)
		h.add(uint64(sign))
		if dump {
			fmt.Printf("%s %d %016x %016x %016x\n", name, i, math.Float64bits(x), b, uint64(sign))
		}
	}
	if !dump {
		fmt.Printf("%s %d %016x\n", name, len(xs), h.h)
	}
}

// unary2 hashes two results per input, as Sincos returns.
func unary2(name string, f func(float64) (float64, float64), xs []float64, dump bool) {
	if wantedSkip(name, dump) {
		return
	}
	h := hasher{14695981039346656037}
	for i, x := range xs {
		a, b := f(x)
		h.add(canon(a))
		h.add(canon(b))
		if dump {
			fmt.Printf("%s %d %016x %016x %016x\n", name, i, math.Float64bits(x), canon(a), canon(b))
		}
	}
	if !dump {
		fmt.Printf("%s %d %016x\n", name, len(xs), h.h)
	}
}

// unaryi hashes an integer result per input, as Ilogb returns.
func unaryi(name string, f func(float64) int, xs []float64, dump bool) {
	if wantedSkip(name, dump) {
		return
	}
	h := hasher{14695981039346656037}
	for i, x := range xs {
		v := uint64(f(x))
		h.add(v)
		if dump {
			fmt.Printf("%s %d %016x %016x\n", name, i, math.Float64bits(x), v)
		}
	}
	if !dump {
		fmt.Printf("%s %d %016x\n", name, len(xs), h.h)
	}
}

// ternary hashes one result per input triple, as FMA takes.
func ternary(name string, f func(float64, float64, float64) float64, xs []float64, dump bool) {
	if wantedSkip(name, dump) {
		return
	}
	h := hasher{14695981039346656037}
	for i, x := range xs {
		j := (i*7 + 3) % len(xs)
		k := (i*13 + 1) % len(xs)
		b := canon(f(x, xs[j], xs[k]))
		h.add(b)
		if dump {
			fmt.Printf("%s %d %016x %016x %016x %016x\n", name, i, math.Float64bits(x), math.Float64bits(xs[j]), math.Float64bits(xs[k]), b)
		}
	}
	if !dump {
		fmt.Printf("%s %d %016x\n", name, len(xs), h.h)
	}
}

// binary32 is binary over f32 conversions, as Nextafter32 takes.
func binary32(name string, f func(float64, float64) float64, xs []float64, dump bool) {
	if wantedSkip(name, dump) {
		return
	}
	h := hasher{14695981039346656037}
	n := 0
	for i, x := range xs {
		js := []int{(i*7 + 3) % len(xs), (i*13 + 1) % len(xs), i}
		for _, j := range js {
			b := canon(f(x, xs[j]))
			h.add(b)
			if dump {
				fmt.Printf("%s %d,%d %016x %016x %016x\n", name, i, j, math.Float64bits(x), math.Float64bits(xs[j]), b)
			}
			n++
		}
	}
	if !dump {
		fmt.Printf("%s %d %016x\n", name, n, h.h)
	}
}

// unaryu32 hashes a u32 conversion per input, as F32bits returns.
func unaryu32(name string, f func(float64) uint64, xs []float64, dump bool) {
	if wantedSkip(name, dump) {
		return
	}
	h := hasher{14695981039346656037}
	for i, x := range xs {
		v := f(x)
		h.add(v)
		if dump {
			fmt.Printf("%s %d %016x %016x\n", name, i, math.Float64bits(x), v)
		}
	}
	if !dump {
		fmt.Printf("%s %d %016x\n", name, len(xs), h.h)
	}
}

func nextafter32f(x, y float64) float64 { return float64(math.Nextafter32(float32(x), float32(y))) }
func f32bitsf(x float64) uint64         { return uint64(math.Float32bits(float32(x))) }
func f32frombitsf(x float64) float64 {
	return float64(math.Float32frombits(uint32(math.Float64bits(x) & 0xffffffff)))
}

// wanted reports whether name is selected: GAUGE_FUNC empty means every function.
func wanted(name string) bool {
	sel := os.Getenv("GAUGE_FUNC")
	return sel == "" || sel == name
}

func wantedSkip(name string, dump bool) bool { return !wanted(name) }

func corpus(dump bool) {
	xs := inputs()
	for _, u := range unaries {
		if !wanted(u.name) {
			continue
		}
		h := hasher{14695981039346656037}
		for i, x := range xs {
			b := canon(u.f(x))
			h.add(b)
			if dump {
				fmt.Printf("%s %d %016x %016x\n", u.name, i, math.Float64bits(x), b)
			}
		}
		if !dump {
			fmt.Printf("%s %d %016x\n", u.name, len(xs), h.h)
		}
	}
	for _, bf := range binaries {
		if !wanted(bf.name) {
			continue
		}
		h := hasher{14695981039346656037}
		n := 0
		for i, x := range xs {
			for _, j := range []int{(i*7 + 3) % len(xs), (i*13 + 1) % len(xs), i} {
				b := canon(bf.f(x, xs[j]))
				h.add(b)
				if dump {
					fmt.Printf("%s %d,%d %016x %016x %016x\n", bf.name, i, j, math.Float64bits(x), math.Float64bits(xs[j]), b)
				}
				n++
			}
		}
		if !dump {
			fmt.Printf("%s %d %016x\n", bf.name, n, h.h)
		}
	}
	unaryi("ilogb", math.Ilogb, xs, dump)
	unarylg("lgamma", math.Lgamma, xs, dump)
	unary2("sincos", math.Sincos, xs, dump)
	unaryu32("f32bits", f32bitsf, xs, dump)
	binary32("nextafter32", nextafter32f, xs, dump)
	ternary("fma", math.FMA, xs, dump)
	// pow with small integer and fractional exponents on positive bases.
	if !wanted("powi") {
		return
	}
	h := hasher{14695981039346656037}
	n := 0
	for _, x := range xs[:2000] {
		for _, y := range []float64{2, 3, -2, 10, -7, 0.25, 1.5, -0.75, 100, 0.3333333333333333, 2.5, -1.5} {
			b := canon(purePow(math.Abs(x), y))
			h.add(b)
			if dump {
				fmt.Printf("powi %d %016x %016x %016x\n", n, math.Float64bits(math.Abs(x)), math.Float64bits(y), b)
			}
			n++
		}
	}
	if !dump {
		fmt.Printf("powi %d %016x\n", n, h.h)
	}
}

func init() {
	if len(os.Args) > 1 && (os.Args[1] == "corpus" || os.Args[1] == "dump") {
		corpus(os.Args[1] == "dump")
		os.Exit(0)
	}
}
