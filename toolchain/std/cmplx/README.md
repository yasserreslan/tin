# cmplx

Complex numbers and the functions of Go's `math/cmplx` (#916).

## API

- Types: `Complex` (two `f64` parts `Re`, `Im`; a value struct), `Complex64` (two `f32` parts), `Complex64.To128()`.
- Constructors and parts: `New`, `From64` (rounds each part to float32, as `complex64(z)` does), `Inf`, `NaN`.
- Predicates and real results: `IsNaN`, `IsInf`, `Abs`, `Phase`, `Polar`.
- Arithmetic and elementary functions: `Conj`, `Rect`, `Sqrt`, `Exp`, `Log`, `Log10`, `Pow`.
- Trigonometric and hyperbolic: `Sin`, `Cos`, `Tan`, `Cot`, `Sinh`, `Cosh`, `Tanh`.
- Inverses: `Asin`, `Acos`, `Atan`, `Asinh`, `Acosh`, `Atanh`.

## Design notes

- The real functions come from `gauge`; the branches, poles, signed zeros, infinities and NaN cases are Go's.
- Each expression keeps Go's shape. arm64 fuses a product into a sum or difference (Go's SSA does too), so a product
  that Go fuses is written inside the sum and one that Go rounds is wrapped in `f64(...)`.
- `Complex` is a value struct: calls allocate nothing.

## Known gaps

- `Pow(0, y)` with a NaN real part and an infinite imaginary part of `y` makes Go panic ("not reached"); this returns NaN.
- `gauge.Log` is not Go's `math.Log` in the last bit for about one input in a thousand (Go's arm64 and amd64 differ too),
  so `Log`, `Log10`, `Pow` and the inverse functions can differ from Go's by one ulp for those inputs. `cmplx_check`
  reports them.
- NaN payloads are not part of a result: the check compares every NaN as NaN.
