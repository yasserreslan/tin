# Stdlib verification against Go (bench/ref/NAME/main.go)

| package | Go reference | result |
|---|---|---|
| mint | strconv | identical on all cases |
| gauge | math, math/bits | the transcendental functions are Tin ports of Go's algorithms: see "gauge: the corpus check" below |
| tide | time | identical; zones and layouts are verified by `bench/ref/tide/zonegen` with `tools/ci/fixtures/tide_zones.tin` (#573): 358,500 lines of `In`, `DateIn`, `Format`, `Parse`, `AddDate`, `Truncate`, `Round` and `ISOWeek` over the issue's nine zones, UTC and `Etc/GMT+5`, at every transition from 1970 to 2040 with the wall times around each, all reference-time layout verbs, fractional seconds and malformed variants. Deliberate difference: a parsed wall time outside 1677–2262 (Go wraps `UnixNano`) is a fault |
| squash (Deflate, Inflate, Gzip, Gunzip, Zlib, Unzlib) | compress/flate, compress/gzip, compress/zlib (`bench/ref/squash`) | identical on 117 lines: Go's and Tin's fixed vectors decoded by both, round trips of 7 inputs at 4 levels in 3 formats, damaged, cut and foreign input, limits. Those vectors do not use every length code, and Tin's encoder and decoder share their tables, so `tools/ci/squash_twin_check.tin` (`bench/ref/squash_twin`, #448) has Go inflate Tin's raw DEFLATE and Tin inflate Go's, at levels 1, 6 and 9, of an input that uses all 29 length codes (both ends of each range) and all 30 distance codes: each of the 59 `lenBase`/`distBase` entries changed by one fails it. Snappy, LZ4 and Zstandard have no Go standard package: checked against the reference tools |
| dice | same algorithms in Go | identical |
| sift | slices/sort, Each (`bench/ref/closures/main.go`) | identical |
| sift (generic: Sort, SortFunc, SortStableFunc, Insert, Delete, Compact, BinarySearch ...) | slices, cmp (`bench/ref/sift/generic.go`) | identical on 149 lines, including the order of equal elements after SortFunc on 119 inputs of 7 shapes up to 20,000 elements; the port is Go's pdqsort, and changing one shift constant in it breaks 16 lines |
| bits | math/bits (`bench/ref/bits`, generated with `toolchain/tests/v2/bits.tin` from one description) | identical on all 37 function hashes: every function at 8, 16, 32 and 64 bits, exhaustive for 8 and 16 bits, about 40,000 values at 32 bits and 20,000 at 64; changing one comparison in `Div64`'s correction loop breaks 2 of them |
| link | net/url (`bench/ref/link`, generated with `toolchain/tests/v2/link.tin` from one description) | identical on all 813 lines: 159 URLs through Parse and 14 through ParseRequestURI (every field and method, fault messages included: bad escapes, IPv6 literals and zones, ports, userinfo, opaque and rootless forms, CTL bytes), 30 strings through the four escape functions, 246 references resolved (50 against the RFC 3986 section 5.4 base, 196 more across 14 bases and 14 references), 23 JoinPath cases, 27 queries and a Values sequence; five mutations (OmitHost, ForceQuery in ResolveReference, the path-segment escape set, the IPv6 group length, the postgres host list) each break 1 to 4 lines |
| glyph (Unicode: `bench/ref/unicode`, generated with `toolchain/tests/v2/unicode.tin` from one description) | unicode, strconv.Quote | identical on all 258 lines: for every rune up to U+10FFFF the 13 classification functions, ToUpper, ToLower, ToTitle and SimpleFold (hash of the results), every one of the 236 category, script and property tables over U+0000 to U+323AF, the orbit cases, the out-of-range edges, and `mint.Quote` and `say.Fmt("%q")` of every valid rune against `strconv.Quote`; five mutations (the upper/lower alternation bit, the stride test, NBSP in IsSpace, the fold orbit lookup, one byte of Zs) each break 1 to 98 lines |
| twine (Unicode and the new functions: `bench/ref/twine`, generated with `toolchain/tests/v2/twine_unicode.tin`) | strings | identical on all 3,544 lines: 38 texts (ASCII, Latin-1, Greek, Turkish, Cyrillic, Arabic, CJK, combining marks, emoji, every kind of space, invalid UTF-8) through ToUpper, ToLower, ToTitle, Title, Fields, FieldsFunc, Map, the Trim family, Index/LastIndexFunc, ContainsFunc, Split/SplitAfter(N), ToValidUTF8, Lines, CutPrefix/Suffix, LastIndexAny, Trim with 11 cutsets, IndexRune with 10 runes, 24 EqualFold pairs and 13 Replacer lists; mutations that remove the Replacer empty-match rule or the fold loop bound hang or break it |
| closures (capture semantics: `bench/ref/closures` and `bench/ref/closures2`, with `toolchain/tests/v2/closures.tin` and `closures2.tin`) | language | identical to Go on loop variables per iteration (3-clause and range), defer with captures, nested closures sharing variables, struct fields and range-var structs, shadowing, `keep` of a closure that captures a kept slice, and `sift.SortFunc` (51 comparisons, as Go), `twine.Map` and `FieldsFunc` with capturing literals; one frame-resident closure measured at 0 pool bytes |
| shapes (`bench/ref/shapes/main.go`, with `toolchain/tests/v2/shapes_dispatch.tin`) | language (interfaces) | identical on all 10 lines: structural `Reader`, `Writer` and composed `ReadWriter`, `Copy[R io.Reader, W io.Writer]` through static specialization, `Log(w dyn io.Writer, ...)` through a Go interface / Tin method table, ordered values constrained by `sift.Ordered`, and `Sized` on `Stack[i64]`. Breaking one constant in the dispatched path breaks the output comparison. `constraints.Any` and `constraints.Comparable` are imported library shapes, covered by `toolchain/tests/v2/constraints.tin`; negatives verify incomparable slices are rejected, bare `any`/`comparable` are not syntax, and bool is outside `sift.Ordered`. The dynamic assembly check finds one indirect call and no allocation call on arm64 and amd64. `bench/dispatch/dyn.tin` reports zero pool bytes; four runs measured a median 4.66 ns/op dynamic and 2.95 ns/op direct on Apple M3 Pro. Missing/mis-signed shape methods include method-name diagnostics and fix-it text. Regression cases for #141 cover the missing method fix-it and request-region escape rejection. Map values and `!dyn` results remain deferred. |
| atlas | maps (`bench/ref/atlas`) | identical; Keys and Values come back in insertion order, so the test sorts them |
| cairn | hand-written Go equivalents | identical |
| trail | path/filepath | identical |
| quarry | os | identical except stderr interleaving and naming fdopendir instead of open in one error |
| lever | flag | identical except: a value that fails to parse leaves the flag unchanged (Go overwrites it with a zero value) |
| twine, glyph | strings, unicode/utf8 | checked by hand against Go semantics |


## Deliberate difference since v0.5 (!T results)

A failing call returns zero values with its fault. Go's strconv returns the clamped value
(MaxInt64, ±Inf, MaxUint64) together with ErrRange; mint.Atoi, ParseInt, ParseUint and
ParseFloat now return 0 with the range fault. toolchain/tests/v2/mint.out records this.


## gauge: the corpus check

`bench/ref/gauge/corpus.go` and `corpus.tin` run every transcendental function over the same deterministic inputs (about 20,000 per function: 70 edge cases plus uniform, log-uniform, raw-bit and quarter-integer values, so NaN, infinities, subnormals and arguments up to 1.8e308 are all in) and print a hash of the result bits, or every result with `dump`. The Go side uses Go's pure algorithms for Exp, Exp2, Sinh, Cosh, Tanh and Pow, because `math.Exp` is assembly on arm64 and amd64 and returns 0 for Exp(-745) where the true value rounds to the smallest subnormal. The Log family calls Go's own `math.Log` and `math.Log1p` on each architecture, which `gauge` matches bit for bit (#942).

    go build -o /tmp/ref ./bench/ref/gauge && tin build bench/ref/gauge/corpus.tin -o /tmp/corpus
    /tmp/ref corpus > go.txt; /tmp/corpus corpus > tin.txt

The two sides print the functions in different orders, so compare them by name (`sort` both files first).

Result (Linux amd64): all 41 hashes are identical. Result (Linux arm64, and the same on macOS arm64): 38 of 41 are identical. The three that differ are log2, expm1 and lgamma, which differ only on arm64, where Go's compiler fuses their multiply-adds in a different way than Tin's (below). Log, Log10, Log1p, Asinh, Acosh, Atanh, Pow and Erfinv are identical on every platform.

The fused multiply-add is the cause. Go's arm64 compiler fuses `x*y + z`, `z + x*y` and `x*y - z` (as FNMSUB) into one instruction that rounds once. Tin's arm64 backend fuses `x*y + z`, `z + x*y` and `z - x*y`, but not `x*y - z`, and it fuses the products it finds in each expression, which need not be the ones Go fuses. So `gauge` writes each function in the shape whose fused products Go's compiler fuses. The disassembly of Go's `math.log`, `log1p`, `asinh`, `acosh` and `atanh` on arm64 gives the shape, `-z + x*y` stands for Go's `x*y - z`, and a product Go rounds is a local (`let p = x*y`), which Tin does not fuse. On amd64 Go fuses nothing, and the same source is unfused, which is Go's amd64 code. The amd64 `Log` is Go's assembly (`log_amd64.s`): its `Frexp` does not normalize a subnormal, and its comparison with Sqrt(2)/2 is `<=`. `gauge.Log` follows both on amd64 (`logAmd64`, `toolchain/std/gauge/log.tin`), so `Log(5e-324)` is Go's amd64 value, -709.09, and not the true -744.44.

`tools/ci/number_check.tin` runs the same functions from `bench/ref/gauge` over about 120,000 inputs each (GAUGE_N, one function at a time) and compares every result bit for bit. On Linux amd64 every function is bit-identical, and expm1 and sincos need no ulp of their allowance. On Linux arm64 and macOS arm64 the same holds for log, log10, log1p, asinh, acosh, atanh, erf, erfc, erfcinv, gamma, erfinv, logb, f32bits, f32frombits, dim, remainder, nextafter, nextafter32 and fma. expm1 has 224 results one ulp off (sincos is allowed one, none used). lgamma goes through Log and Sin and cancels near its zero crossings, so it is checked with a 1e-14 absolute plus 1e-13 relative tolerance: 119,975 of 120,077 results are bit-identical on arm64, with a worst relative difference of 2.16e-16. Its Go twin uses `math.Log`, which is the same function as `gauge.Log` on each architecture. The special values are pinned in `toolchain/tests/v2/gauge_more.tin` and `toolchain/tests/v2/gauge_log.tin`, which agree with Go's line for line.

Error against exact arithmetic (Python `decimal`, 800 digits for log1p), in ulps, on the inputs where the result is a normal number. The table was measured on macOS arm64 before #942 and is kept as measured. Since #942 the corpus hashes of log, log10, log1p and pow agree with Go on every platform (above), so for those rows Tin now equals Go.

| function | inputs | Tin = Go | max Tin | max Go | mean Tin | mean Go |
|---|---|---|---|---|---|---|
| log | 10,110 | 99.80% | 0.718 | 0.725 | 0.2509 | 0.2510 |
| log2 | 10,110 | 95.75% | 35.1 | 34.1 | 0.2776 | 0.2767 |
| log10 | 10,110 | 99.81% | 1.76 | 1.61 | 0.3780 | 0.3780 |
| log1p | 12,875 | 99.93% | 0.707 | 0.707 | 0.2034 | 0.2034 |
| pow (4,000 sampled) | 3,476 | 99.94% | 141 | 141 | 4.209 | 4.209 |

The large maximum of Log2 is Go's algorithm, not the port: Log2 loses precision for x near 1 (it adds Log(frac)/ln2 to an exponent of 1), and Pow's error grows with the size of the result. They are reproduced faithfully. A more accurate Log2 and Pow would be a Tin improvement on Go, and the corpus would then show them as deliberate differences.
