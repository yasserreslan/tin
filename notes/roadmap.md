# Roadmap

## v0.3 (done 2026-10-01): a usable, documented language
1. Land in-flight work (local imports, partial stdlib packages, agent-reported compiler bugs). Done.
2. Robustness: panic backtraces, bounds failures with index and length, defer. Done.
3. Remaining stdlib: stamp, seal, ore, flume, herald, wire, crucible. Done.
4. Docs: docs/LANGUAGE.md, docs/STDLIB.md (generated), README. Done.
5. Final benchmarks (README). Done.

## Linux port (done 2026-10-01)
linux-arm64 and linux-amd64 pass every suite and self-host; CI runs all three targets
natively. All review issues (#1–#21, #43, #44) are fixed.

## v0.5: syntax (notes/syntax_v05.md; phases 2–8 of #46)
Go syntax kept; adds `!T`/`fail`/`catch`, call-site `mut`, `for i := range n`, the
capitalized-export rule, string interpolation, enums with exhaustive `switch`, more map key
types with insertion order, `f32`, and Go-style `*_test.tin` tests.

## v0.4: non-blocking I/O inside handlers (design in notes/design_v04.md; phases 9–13 of #46)
From the user's spec (2026-10-01):
- Stackful tasks per in-flight request on each core; waiting on a socket registers the fd with
  the core's kqueue and switches to the event loop; resume when ready. Pooled task stacks with
  guard pages; cap in-flight requests per core (backpressure).
- Request pool moves from per-core to per-request; each request's pool is wiped when it
  finishes; the region checker still forbids escapes without keep().
- Non-blocking outbound TCP client; Redis client (pooled per core, pipelined); MySQL client
  (wire protocol, prepared statements, pooled per core); every call takes a timeout and a
  deadline cancels the request.
- Small pool of blocking helper threads (file I/O, DNS) reporting back via relay.
Done when: a 100ms-wait handler on a core does not delay fast requests on that core; GET
/users/{id} via Redis cache backed by MySQL, wrk2 vs the same service in Go with chi: Tin wins
on req/s and p99, including a slow/fast mix. make bootstrap stays a fixed point, tests green.
Show the design (task switching, stack size, pool ownership) before building.

Status (2026-10-02): phases 9–12 and the websocket package are merged (#64–#73): request
tasks, non-blocking I/O and helper threads, `query`, redis, mysql, websocket. Phase 13's
service and harness are in bench/v04 (Tin and Go + chi, wrk2); the final measurement on a
quiet machine is still to do (see the open benchmark issue).

## Foundations (decided 2026-10-02): notes/design_foundations.md
Closures that capture, shapes, fault chains and `guard`, tasks and scopes with `context` as ambient
deadline and cancellation, atomics, compile-time type information, packages with capabilities. The
document decides each, with what it replaces, what was rejected and what it unlocks, and orders the
nine steps by dependency. docs/COVERAGE.md is the checklist of what each adds.


## Ten-year plan (written 2026-10-02)

Tin's goal: a language as ready as Go, Rust and Odin for servers and tools, written in Tin, with
performance as the end goal. This is the whole list, ordered by phase; a box is checked when it is merged
with its tests (and, for library work, verified against Go: see notes/stdlib_verified.md). Years are targets,
not promises; the order inside a phase follows dependencies (notes/design_foundations.md). The standard
library checklist (section 9) is generated from docs/COVERAGE.md, which stays the detailed inventory.

### 1. Language (year 1 to 3)

Foundations, in the order of notes/design_foundations.md (#140 to #146):
- [ ] Closures that capture, as region objects with cells (#140)
- [ ] Shapes: structural interfaces, static by default, `dyn` for dynamic dispatch (#141)
- [ ] Fault chains: `fault.Is`, `Wrap`, `Join`, `try ... wrap`, sentinels, `guard` in place of `recover` (#142)
- [ ] Tasks and scopes: `scope`, `spawn`, `lane[T]` (structured concurrency on a core) (#143)
- [ ] `context` as ambient deadline, cancellation and slots: `within`, `slot`/`with` (#144)
- [ ] Derivation and attributes in place of `reflect` and struct tags (#145)
- [ ] Packages with `tin.lock`, content addressing and capabilities (#146)
- [ ] The `io` family on shapes (Reader, Writer, Closer, Seeker, ReaderAt, WriterTo)
- [ ] Atomics for cross-core counters and flags (no mutexes)

Other language work:
- [ ] Range over functions and iterators (`iter.Seq` equivalent) and the `*Seq` forms in the library
- [ ] Struct embedding with promotion
- [ ] Method values and method expressions
- [ ] Named results and bare `return`
- [ ] User-declared variadic functions
- [ ] `min` and `max` of any number of arguments, `clear`
- [ ] Value arrays `[N]T` (copy semantics) next to the slice form
- [ ] Full slice expressions `s[a:b:c]`
- [ ] Labels, `goto`-free labeled `break` and `continue`, `fallthrough` decision
- [ ] `init` functions decision and documentation
- [ ] Generic constraints: named constraints, `cmp.Ordered`, constraints with methods (after shapes), generic type aliases
- [ ] Generic methods and higher-kinded helpers decision
- [ ] Complex numbers decision (`complex128`, `math/cmplx`) or a documented omission
- [ ] `//embed` (compile-time file embedding) and generate hooks
- [ ] `unsafe`-equivalent: a small, audited raw-memory package for the standard library only
- [ ] Integer overflow policy (wrapping vs checked, per type) written down and tested
- [ ] Optionals: `?T` for non-reference types, `if let`-style binding sugar decision
- [ ] Pattern matching: nested patterns, guards, `switch` on tuples
- [ ] Const evaluation: compile-time functions, typed const generics
- [ ] Formal grammar and a language specification document, versioned
- [ ] Compatibility promise and edition mechanism (breaking changes only by edition)

### 2. Compiler (year 1 to 5)

Correctness:
- [ ] A conformance suite: Tin's own spec tests, and the corpus of Go programs ported as differential tests
- [ ] Differential fuzzing of the compiler against the interpreter-free reference outputs (Go twins)
- [ ] Fuzzing the lexer, parser and checker (no crash on any input, errors with positions)
- [ ] LLVM-style test coverage: every pass has lit-style tests for what it must and must not do
- [ ] Error messages: every diagnostic has a code, a span, a suggestion where one exists
- [ ] Compile-time region checker proofs for the documented escape rules; counterexample corpus
- [ ] Reproducible builds: the same source gives the same bytes on every machine

Performance of the generated code:
- [ ] Inliner for multi-statement functions (the cause of the remaining math and sort gaps)
- [ ] Intrinsics: clz, ctz, popcount, rbit, rev, fma, sqrt, rounding, multiply-high, add-with-carry
- [ ] Constant hoisting and strength reduction; loop-invariant code motion
- [ ] Bounds-check elimination from range and induction facts
- [ ] Register allocation: linear scan to a graph or SSA-based allocator
- [ ] An SSA middle end (or equivalent) with common subexpression elimination and dead store elimination
- [ ] Escape and region analysis that stack-allocates request-local values
- [ ] SIMD: vector types, auto-vectorization of simple loops, hand-written kernels for memchr, memcmp, hashing, base64, utf8
- [ ] Profile-guided optimization (inlining and layout from a recorded profile)
- [ ] Link-time optimization across packages; dead data and code stripping
- [ ] Monomorphization cost control: shared instantiation for identical machine code
- [ ] Compile speed: incremental and parallel builds, a build cache keyed by content
- [ ] Debug info: DWARF line tables and variables, so debuggers and profilers work

Targets:
- [ ] linux-arm64, linux-amd64, darwin-arm64 at full parity (done for the first three: keep green)
- [ ] darwin-amd64
- [ ] Windows (amd64, arm64): PE writer, Win32 runtime part, IOCP
- [ ] FreeBSD and OpenBSD (kqueue is shared with darwin)
- [ ] riscv64
- [ ] WebAssembly (wasm32-wasi first, then browser)
- [ ] Embedded and freestanding targets (no OS runtime part)
- [ ] A second backend (LLVM IR emitter) as an optimization-quality reference, not a dependency

### 3. Runtime (year 1 to 5)

- [ ] Allocator: size classes, per-core caches, huge-page support, no libc (libc removal phases 2 to 6, #125)
- [ ] Linux without libc: raw syscalls everywhere, vDSO clock
- [ ] Number parsing and printing without libc (strtod, snprintf replacements)
- [ ] DNS resolver in Tin (stub resolver, /etc/hosts, search domains, happy eyeballs)
- [ ] io_uring backend next to epoll and kqueue
- [ ] Task scheduler: work stealing across cores as an opt-in, fairness, priority by deadline
- [ ] Preemption points and a watchdog for tasks that never yield
- [ ] Stack growth for tasks (guard pages now; segmented or copied stacks later)
- [ ] Timers: hierarchical wheel, monotonic clocks, timer slack
- [ ] Panics: backtraces with symbolized frames in release builds, core dumps, crash reports
- [ ] Memory limits per request, per core and per process, with backpressure
- [ ] Observability built in: counters, histograms, trace spans, the pprof format
- [ ] Signals, process control, fork-free spawn, pipes, sockets options, `sendfile`, `splice`
- [ ] TLS in the runtime's I/O path (non-blocking handshake, session resumption, kTLS)
- [ ] Graceful reload and zero-downtime restart (socket handoff)

### 4. Performance programme (continuous)

- [ ] A benchmark suite in the repo with Go, Rust and Odin baselines, run in CI on dedicated hardware, regressions block merges
- [ ] The service benchmark of v0.4 measured on a quiet machine (wrk2: req/s and p99 against Go with chi), published
- [ ] Per-package micro benchmarks: strings, strconv, math, sort, maps, json, http parsing, hashing, compression
- [ ] Performance budget documented per stdlib package (allocations per call, bytes copied)
- [ ] JSON: faster than the Go standard library and than `sonic`-class libraries on real-world payloads
- [ ] HTTP server: beat Go's net/http and fasthttp on throughput and tail latency, with TLS
- [ ] Database clients: fewer allocations than pgx and go-sql-driver at equal features
- [ ] Startup time and binary size budgets enforced in CI

### 5. Tooling (year 1 to 6)

- [ ] `tin` command: build, run, test, bench, fmt, vet, doc, get, mod, clean, install, version, env
- [ ] `tin fmt`: one canonical format, stable, with a style check in CI
- [ ] `tin vet`: the compiler's lints (unused, shadowing, suspicious formatting, misuse of `keep`)
- [ ] `tin doc`: documentation from comments, searchable, with examples that run as tests
- [ ] `tin test`: table tests, subtests, parallel tests, golden files, coverage, race-free by construction, fuzzing, benchmarks with `benchstat`-style comparison
- [ ] Language server (LSP): diagnostics, completion, go-to-definition, rename, hover, code actions, inlay hints
- [ ] Debugger support: DWARF, `lldb` and `gdb` pretty-printers, a Tin-aware debugger adapter (DAP)
- [ ] Profilers: CPU, heap (pool usage), task and latency traces; flame graphs; `tin pprof`
- [ ] Package manager: resolution, lock file, checksums, vendoring, private registries, offline mode, mirror (#146)
- [ ] Package registry and documentation site
- [ ] Editor support: VS Code, JetBrains, Neovim, Helix, Zed; tree-sitter grammar; syntax highlighting everywhere GitHub renders
- [ ] Playground in the browser (wasm)
- [ ] Code generation tools in Tin (stringer-like, mocks via shapes, protobuf, OpenAPI, SQL)
- [ ] Migration tooling: a Go-to-Tin translator for the common subset, and a report of what does not map
- [ ] Continuous integration templates and a hosted build cache
- [ ] Installers: Homebrew, apt and rpm repositories, container images, `tinup` for toolchain versions

### 6. Safety and security (continuous)

- [ ] A memory-safety statement and proof sketch for the region checker; external review
- [ ] Sanitizer-grade debug mode (poisoned pools, canaries, use-after-reset detection at run time)
- [ ] Fuzz every parser in the library (JSON, HTTP, URL, TLS records, database wire protocols, archive formats)
- [ ] Constant-time guarantees for the crypto package, tested with timing harnesses
- [ ] Supply chain: signed releases, reproducible builds, SBOM, lock-file verification, capability prompts for dependencies
- [ ] Vulnerability policy, advisory database and `tin audit`
- [ ] FIPS-style validated crypto build, if customers need it

### 7. Ecosystem and community (year 2 to 10)

- [ ] Language specification, tour, book, cookbook, migration guide from Go
- [ ] Style guide and project layout guide
- [ ] Governance: RFC process, editions, release cadence, long-term support releases
- [ ] A foundation or an equivalent home for the project; trademark and license policy
- [ ] Third-party libraries to encourage: web frameworks, ORMs, queue clients (Kafka, NATS, SQS, RabbitMQ), cloud SDKs, observability agents, gRPC, GraphQL
- [ ] Production references: Large production services running in Tin, with published numbers
- [ ] Conferences, training material and certification
- [ ] Compatibility with Go tooling where useful (`go list`-like metadata, SARIF reports)

### 8. Years, in short

| Year | Theme | Exit criteria |
|---|---|---|
| 1 (2026 to 2027) | Foundations and breadth | Sections 1 foundations done; stdlib "high demand" packages done and verified; libc gone on Linux; the service benchmark published |
| 2 | Production readiness | `context`, TLS, database/sql shape, HTTP client and server complete; LSP and formatter; first external service in production |
| 3 | Performance leadership | SSA middle end, inliner, intrinsics, SIMD kernels; Tin ahead of Go on the benchmark suite; package manager and registry live |
| 4 | Platforms | Windows, darwin-amd64, wasm; debugger and profiler complete |
| 5 | Ecosystem | Third-party libraries for the usual stacks; spec 1.0 and the compatibility promise |
| 6 to 8 | Depth | Second backend, PGO and LTO by default, formal checker proofs, FIPS build, embedded targets |
| 9 to 10 | Maturity | Long-term support, foundation governance, the standard library at parity with Go's and ahead where Tin's design allows |

### 9. Standard library, A to Z

One box per Go standard-library package (the 176 that `go list std` reports for Go 1.26, as in docs/COVERAGE.md), ordered by import path. `[x]` means done; the note says what Tin package carries it and what is still missing.

- [ ] `archive/tar`: missing; tar archives; needs the io shape first
- [ ] `archive/zip`: missing; zip archives; needs compress/flate
- [ ] `bufio` (flume): partial; buffered Reader (Line, Byte, ReadAll) and Writer (Str, Int, Flush); no Scanner with split functions, no ReadWriter
- [ ] `bytes` (ore, twine.Builder): partial; about 20 functions on []u8; no Buffer or Reader type, no Map, Title, FieldsFunc or TrimFunc
- [ ] `cmp` (sift (Less, Cmp), builtin min/max): partial; Less and Cmp (NaN first, as Go); no cmp.Or and no named Ordered constraint (a union such as `i64
- [ ] `compress/bzip2`: missing
- [ ] `compress/flate`: missing; needed by gzip, zlib and zip
- [ ] `compress/gzip`: missing
- [ ] `compress/lzw`: missing
- [ ] `compress/zlib`: missing
- [ ] `container/heap` (cairn): partial; IntHeap and IntMaxHeap; no heap over any element type (generics now allow one)
- [ ] `container/list` (cairn (deque, queue)): missing; no doubly linked list with stable element handles
- [ ] `container/ring`: missing
- [ ] `context` (design: ambient task deadline and cancellation): missing; every request task already has a deadline; the cancel signal, values and the scoped form are unbuilt
- [-] `crypto`: n/a; the Hash registry and interfaces; there are no interfaces
- [ ] `crypto/aes`: missing
- [ ] `crypto/cipher`: missing; GCM, CTR, CBC
- [ ] `crypto/des`: missing
- [ ] `crypto/dsa`: missing; deprecated in Go
- [ ] `crypto/ecdh`: missing
- [ ] `crypto/ecdsa`: missing
- [ ] `crypto/ed25519`: missing
- [ ] `crypto/elliptic`: missing
- [-] `crypto/fips140`: n/a; Go's FIPS module switch
- [ ] `crypto/hkdf`: missing
- [ ] `crypto/hmac` (seal): partial; HmacSha256 only
- [ ] `crypto/hpke`: missing
- [ ] `crypto/md5` ((postgres/md5, internal)): partial; exists only inside the PostgreSQL client; not public
- [ ] `crypto/mlkem`: missing
- [-] `crypto/mlkem/mlkemtest`: n/a
- [ ] `crypto/pbkdf2` (seal): partial; Pbkdf2Sha256 and a timeout form; no other hashes
- [ ] `crypto/rand` (seal): partial; RandomBytes; no Reader, Int or Prime
- [ ] `crypto/rc4`: missing; deprecated in Go
- [ ] `crypto/rsa` (seal): partial; ParseRSAPublicKeyPEM and EncryptOAEPSha1 (the MySQL login); no key generation, signing or verification
- [ ] `crypto/sha1` (seal): partial; Sha1 one shot; no streaming hash
- [ ] `crypto/sha256` (seal): partial; Sha256, Sha256Hex (hardware instructions where present); no streaming hash, no SHA-224
- [ ] `crypto/sha3`: missing
- [ ] `crypto/sha512`: missing
- [ ] `crypto/subtle` (seal): partial; ConstantTimeEq only
- [ ] `crypto/tls`: missing; issue #124: client first, then server; blocks https, wss and TLS to databases
- [ ] `crypto/x509`: missing
- [ ] `crypto/x509/pkix`: missing
- [ ] `database/sql` (mysql, postgres (and the `query` type)): partial; each client has Open, Query, Exec, Begin, Commit, Rollback and typed values, pooled per core; no shared driver abstraction, no Scan into structs, no prepared-statement handle API
- [ ] `database/sql/driver`: design; no interfaces: a driver would be a package with a fixed shape or a table of functions
- [ ] `debug/buildinfo`: missing; low priority
- [ ] `debug/dwarf`: missing; low priority
- [ ] `debug/elf`: missing; low priority (the compiler writes ELF but does not read it)
- [-] `debug/gosym`: n/a; Go symbol tables
- [ ] `debug/macho`: missing; low priority
- [ ] `debug/pe`: missing; low priority
- [-] `debug/plan9obj`: n/a
- [ ] `embed`: missing; compile-time file embedding
- [ ] `encoding` (compile-time derivation): design; Marshaler interfaces become derived code, as argo already does for JSON
- [ ] `encoding/ascii85`: missing
- [ ] `encoding/asn1`: missing; needed by x509
- [ ] `encoding/base32`: missing
- [ ] `encoding/base64` (seal): partial; standard (padded) and URL-safe (unpadded) with decoders; no padded URL-safe form, no unpadded standard form, no streaming encoder
- [ ] `encoding/binary`: missing; byte orders, varints, Read and Write of fixed-size values
- [ ] `encoding/csv`: missing
- [ ] `encoding/gob`: missing; low priority: Go's own format
- [ ] `encoding/hex` (seal): partial; Hex and HexDecode; no Dump, no streaming
- [ ] `encoding/json` (argo): partial; Put and Get are generated per type, fast; no decoding into a dynamic value, no field tags, no Indent, no streaming Encoder or Decoder, no RawMessage beyond Raw
- [ ] `encoding/pem`: missing
- [ ] `encoding/xml`: missing
- [ ] `errors` (fault, try, catch, say.Fault): partial; no Is, As, Unwrap or Join: a fault is a message, with no wrapping chain
- [ ] `expvar`: missing
- [ ] `flag` (lever): partial; Str, Int, Bool, F64, Parse, Usage; no FlagSet, no Duration, no custom Value
- [ ] `fmt` (say): partial; Line, Fmt, Str, Fault and string interpolation with format specs, by static type; no Sscanf, Fscan or Scan, and no Stringer or Formatter (formatting is derived)
- [-] `go/ast`: n/a; Go's own compiler front end; Tin's compiler is selfhost/
- [-] `go/build`: n/a
- [-] `go/build/constraint`: n/a
- [-] `go/constant`: n/a
- [-] `go/doc`: n/a
- [-] `go/doc/comment`: n/a
- [-] `go/format`: n/a; a Tin formatter is a separate tool, not this package
- [-] `go/importer`: n/a
- [-] `go/parser`: n/a
- [-] `go/printer`: n/a
- [-] `go/scanner`: n/a
- [-] `go/token`: n/a
- [-] `go/types`: n/a
- [-] `go/version`: n/a
- [ ] `hash`: design; the Hash interface; streaming hashes need a generic or a table of functions
- [ ] `hash/adler32` (stamp): partial; Adler32 one shot
- [ ] `hash/crc32` (stamp): partial; Crc32, Crc32C, Crc32Update; no table type, no streaming hash
- [ ] `hash/crc64`: missing
- [ ] `hash/fnv` (stamp): partial; Fnv32a and Fnv64a; not the FNV-1 variants
- [ ] `hash/maphash`: missing; maps are hashed internally with a per-process key
- [ ] `html`: missing; EscapeString and UnescapeString
- [ ] `html/template`: missing; contextual escaping; templ-style code generators are the common alternative
- [ ] `image`: missing
- [ ] `image/color`: missing
- [ ] `image/color/palette`: missing
- [ ] `image/draw`: missing
- [ ] `image/gif`: missing
- [ ] `image/jpeg`: missing
- [ ] `image/png`: missing; needs compress/zlib
- [ ] `index/suffixarray`: missing
- [ ] `io` (flume (concrete Reader and Writer)): design; Reader and Writer are interfaces in Go; Copy, Pipe, MultiWriter, LimitReader and TeeReader have no Tin form yet
- [ ] `io/fs`: missing
- [-] `io/ioutil` (quarry): n/a; deprecated in Go; quarry has ReadFile, WriteFile, ReadDir
- [ ] `iter`: missing; range over functions; `for range` covers slices, strings, maps and integers
- [ ] `log` (herald): partial; levels, output, clock; no Logger values
- [ ] `log/slog` (herald): partial; leveled lines with key and value pairs; no Handler, Group or LogValuer
- [ ] `log/syslog`: missing
- [ ] `maps` (atlas): partial; Keys, Values (slices, in insertion order), SortedKeys, Clone, Copy, Equal, EqualFunc, DeleteFunc; no iterator forms (All, Insert, Collect)
- [ ] `math` (gauge): partial; Sin to Atan2, Sinh to Tanh, Exp, Exp2, Log family, Pow, Cbrt, Hypot, Mod, Frexp, Ldexp, Modf (ported from Go, no libm); missing Gamma, Lgamma, Erf, Erfc, Expm1, Asinh, Acosh, Atanh, Sincos, FMA, Ne...
- [ ] `math/big`: missing; Int, Float, Rat
- [x] `math/bits` (bits): done; LeadingZeros, TrailingZeros, PopCount (OnesCount), Len, RotateLeft, Reverse, ReverseBytes, and Add, Sub, Mul, Div, Rem with carries, at 8, 16, 32 and 64 bits as Go has them, each name carrying its ...
- [ ] `math/cmplx`: missing; there is no complex type
- [ ] `math/rand` (dice): partial; xoshiro256** generators, Intn, F64, NormF64, Perm, Shuffle; no Zipf, no ExpFloat64, no Source interface
- [ ] `math/rand/v2` (dice): partial; same generators; no PCG or ChaCha8 types, different method names
- [ ] `mime`: missing
- [ ] `mime/multipart`: missing
- [ ] `mime/quotedprintable`: missing
- [ ] `net` (wire): partial; TCP Dial, DialTimeout, Listen, Accept, deadlines; no UDP, Unix sockets, IP or CIDR types, resolver control
- [ ] `net/http` (anvil, wire, websocket): partial; server with Router, middleware, groups, HEAD and 405 handling; client Get, Post, Do; WebSocket; no TLS, HTTP/2, cookies, multipart, Client or Transport configuration, streaming bodies
- [ ] `net/http/cgi`: missing; low priority
- [ ] `net/http/cookiejar`: missing
- [ ] `net/http/fcgi`: missing; low priority
- [ ] `net/http/httptest` (anvil.Router.Run): partial; runs a request through a router without a socket; no ResponseRecorder or test Server
- [ ] `net/http/httptrace`: missing
- [ ] `net/http/httputil`: missing; ReverseProxy, DumpRequest
- [ ] `net/http/pprof`: missing; profiling endpoints; part of the performance goal
- [ ] `net/mail`: missing
- [ ] `net/netip` (link): partial; only the check that a bracketed URL host is an IPv6 address, with Go's fault messages (private to link); no Addr, Prefix or AddrPort types
- [ ] `net/rpc`: missing; low priority
- [ ] `net/rpc/jsonrpc`: missing; low priority
- [ ] `net/smtp`: missing
- [ ] `net/textproto`: missing
- [ ] `net/url` (link): partial; Parse, ParseRequestURI, URL (String, EscapedPath, EscapedFragment, Hostname, Port, RequestURI, Redacted, ResolveReference, Parse, JoinPath), Userinfo, Values (Get, Set, Add, Del, Has, Encode), Pars...
- [ ] `os` (quarry): partial; Args, environment, ReadFile, WriteFile, AppendFile, Mkdir, Remove, Rename, ReadDir, Getwd, Exit, Hostname, Pid; files are opened through flume (buffered) and there is no os.File type with Seek; no ...
- [ ] `os/exec`: missing
- [ ] `os/signal`: missing; anvil handles SIGTERM and SIGINT for graceful shutdown internally
- [ ] `os/user`: missing
- [ ] `path` (trail): partial; Clean, Base, Dir, Ext, Join, Split, Match, IsAbs
- [ ] `path/filepath` (trail): partial; the same, plus Rel; no Walk, WalkDir, Glob, Abs or EvalSymlinks
- [ ] `plugin`: design; no dynamic loading
- [ ] `reflect` (compile-time derivation (argo, say)): design; runtime reflection is not planned; what code uses it for (serialization, validation, mapping rows to structs) becomes derived code
- [ ] `regexp`: missing; needs an RE2-style engine; linear time
- [ ] `regexp/syntax`: missing
- [ ] `runtime` (hearth): partial; Cores, ID, MemLimit, PoolChunk, Reset; no GC controls (there is no GC), no Gosched or NumGoroutine, no Caller or Stack
- [-] `runtime/cgo`: n/a
- [ ] `runtime/coverage`: missing
- [ ] `runtime/debug`: missing; backtraces exist on panic; no SetGCPercent (no GC), no Stack or ReadBuildInfo
- [ ] `runtime/metrics`: missing
- [ ] `runtime/pprof`: missing; CPU and allocation profiling; part of the performance goal
- [-] `runtime/race`: n/a; the language rules out shared mutable state between threads
- [ ] `runtime/trace`: missing
- [ ] `slices` (sift): partial; Sort, SortFunc, SortStableFunc (Go's algorithm, same order of equal elements), IsSorted, BinarySearch, Min, Max, Index, Contains, Equal, Compare, Reverse, Insert, Delete, DeleteFunc, Replace, Compa...
- [ ] `sort` (sift): partial; Ints, Strs, SortBy, Search* and the generic Sort and SortFunc; no sort.Interface (by design), no sort.Slice (use SortFunc)
- [ ] `strconv` (mint): partial; Itoa, Atoi, ParseInt, ParseUint, ParseBool, ParseFloat, FormatInt, FormatUint, FormatFloat, Quote, Unquote and friends; no AppendFloat, AppendBool, QuoteToASCII, IsPrint, ParseComplex
- [ ] `strings` (twine): partial; every function except the iterator forms and Reader: Index family, Split family with SplitAfter, Fields and FieldsFunc, Map, Title, Unicode ToUpper, ToLower, ToTitle, EqualFold by SimpleFold, Trim ...
- [-] `structs`: n/a
- [ ] `sync` (share-nothing cores, relay): design; no Mutex or RWMutex by design; WaitGroup, Once, Pool and Map need routine-level equivalents
- [ ] `sync/atomic`: missing; the runtime has atomic operations as compiler intrinsics; there is no public package
- [ ] `syscall`: missing; low priority
- [ ] `testing` (crucible, `tin test`): partial; checks, Run, benchmarks; no t.Parallel, subtests with cleanup, TempDir, fuzzing, example tests
- [ ] `testing/cryptotest`: missing
- [ ] `testing/fstest`: missing
- [ ] `testing/iotest`: missing
- [ ] `testing/quick`: missing
- [ ] `testing/slogtest`: missing
- [ ] `testing/synctest`: missing
- [ ] `text/scanner`: missing
- [ ] `text/tabwriter`: missing
- [ ] `text/template`: missing
- [ ] `text/template/parse`: missing
- [ ] `time` (tide): partial; Now, Since, Sleep, Wait, durations with parse and format, RFC 3339 and HTTP date, calendar arithmetic; no time zones or Location, no layout-based Format and Parse, no Timer, Ticker or After, no Mon...
- [ ] `time/tzdata`: missing
- [ ] `unicode` (glyph): partial; Unicode 15.0.0 as Go has it: Is over every category, script and property (the Table enum), IsOneOf, IsLetter, IsDigit, IsNumber, IsSpace, IsUpper, IsLower, IsTitle, IsMark, IsPunct, IsSymbol, IsCon...
- [ ] `unicode/utf16`: missing
- [x] `unicode/utf8` (glyph): done; every function
- [ ] `unique`: missing
- [ ] `unsafe`: design; raw operations exist only for the standard library, behind the region checker
- [-] `weak`: n/a; there is no garbage collector

`[-]` marks a package that is specific to Go's toolchain or runtime and has no counterpart to build.

### 10. Beyond Go's library (Tin-native)

- [ ] `link`, `atlas`, `bits`, `sift`, `glyph`: keep API parity with Go and add Tin-native forms (iterators, shapes, derivation)
- [ ] `herald`: structured logging on par with slog, with sampling and async sinks
- [ ] Queue and stream clients: Kafka, NATS, SQS, RabbitMQ, Redis streams
- [ ] Cloud clients: S3-compatible object storage, GCS, Azure Blob; secrets managers; service discovery
- [ ] gRPC and protobuf, with derivation instead of reflection
- [ ] GraphQL, OpenAPI server and client generation
- [ ] Rate limiting, circuit breaking, retries with budgets, hedged requests
- [ ] Caching: in-process cache with admission policy, distributed cache clients
- [ ] Metrics and tracing: Prometheus, OpenTelemetry, Datadog formats, exemplars
- [ ] Config: layered configuration, environment, files, hot reload, secrets
- [ ] Feature flags and experimentation clients
- [ ] Template engines (text and HTML, escaped by construction)
- [ ] Search and analytics clients (Elasticsearch, ClickHouse)
- [ ] Audio and media helpers relevant to streaming services (ID3, MP4 boxes, HLS and DASH manifests, Opus/AAC framing)
- [ ] Machine-learning inference helpers (tensor layout, ONNX runtime binding) once FFI policy is settled

### 11. Open questions (decide, then move to a section above)

- [ ] FFI policy: how Tin calls C libraries without `cgo`, and how capabilities limit it
- [ ] Generics beyond monomorphization: dictionary passing for code size
- [ ] Whether `dyn` shapes need a stable ABI across packages
- [ ] Async I/O across cores for file systems that cannot be non-blocking
- [ ] Hot code reload for development
- [ ] A garbage-collected arena type for long-lived graphs that `keep` copies poorly

### 12. Detailed task breakdown

Every item below is a task a pull request can close. Language features share one template; each feature lists the specific work on top of it.

#### 12.1 The template every language feature follows

- [ ] design note updated in notes/design_foundations.md with the final syntax and rules
- [ ] lexer and parser: grammar, error recovery, positions in diagnostics
- [ ] checker: typing rules, diagnostics with codes, region/escape rules
- [ ] lowering and code generation on arm64 and amd64
- [ ] runtime support (if any), with allocation and Linux behavior covered
- [ ] tests: positive, negative (`_bad.tin` with `.err`), regression cases tied to issues
- [ ] docs: docs/LANGUAGE.md section, COVERAGE.md row, examples
- [ ] stdlib adoption: the packages that wait for it (named below) switch to it
- [ ] performance check: benchmark against the Go equivalent, no regression in compile time

#### 12.2 Language features

**Closures that capture (#140)**
- [ ] closure object layout: code pointer plus captured cells, region-tagged
- [ ] capture analysis: by value for immutable, cell for mutated variables
- [ ] `keep` of a closure copies its cells into long-lived memory
- [ ] escape rule: a closure that captures request memory cannot be stored in a global
- [ ] function literals as arguments to `sift.SortFunc`, `twine.Map`, `FieldsFunc` etc. with capture
- [ ] defer with closures; `for` loop variable per-iteration semantics decided and documented

**Shapes (#141)**
- [ ] structural shapes declared with `shape`, satisfied implicitly by methods
- [ ] static dispatch by monomorphization (default) with no run-time cost
- [ ] `dyn` fat reference (data pointer plus table) for open sets, with region rules
- [ ] shape embedding and composition; shapes with generic parameters
- [ ] error cases: missing method, wrong signature, mut mismatch, with fix-it text
- [ ] port `io.Reader`/`Writer`/`Closer`/`Seeker`, `sort.Interface`, `hash.Hash`, `fmt.Stringer`, `database/sql/driver` onto shapes
- [ ] `say` and `argo` honor `String()` and derived encoders through shapes

**Fault chains and guard (#142)**
- [ ] fault values with a cause and a message; sentinels declared with `fault`
- [ ] `fault.Is`, `fault.As`-equivalent by shape, `Wrap`, `Join`, `Unwrap`
- [ ] `try ... wrap "context"` sugar; message format `outer: inner`
- [ ] `guard` blocks that turn a panic into a fault at a task boundary (replaces `recover`)
- [ ] stack traces attached to faults in debug builds
- [ ] port every stdlib error: `url.Error`, `strconv.NumError`, `os.PathError`, `net.OpError`
- [ ] argo and herald render fault chains

**Tasks and scopes (#143)**
- [ ] `scope { ... }` waits for every `spawn`ed task; cancellation propagates down
- [ ] `spawn f(x)` on the same core; `lane[T]` bounded queues between tasks
- [ ] result collection: `scope.Wait`, first-error cancellation, `errgroup` equivalent
- [ ] cross-core send with `relay` shapes; ownership transfer rules for pool memory
- [ ] deadlock and leak diagnostics in debug builds
- [ ] task-local storage via context slots
- [ ] conformance tests: ordering, cancellation, panics inside tasks, timers

**Context (#144)**
- [ ] ambient deadline and cancellation per task: `within(duration) { ... }`
- [ ] `slot[T]` declarations and `with slot = value { ... }` scopes
- [ ] every blocking call (sockets, timers, queries, sleeps) observes the ambient deadline
- [ ] `context.Context`-style parameters are not needed: migration guide for Go-style code
- [ ] interop shim exposing `Done()`, `Err()`, `Value()` for ported libraries
- [ ] herald, trace and database clients read slots (request id, trace id)

**Derivation and attributes (#145)**
- [ ] compile-time type information: field names, types, offsets, enum variants
- [ ] `#[json(name=...)]`-style attributes checked by the compiler
- [ ] user-defined derivers written in Tin and run at compile time
- [ ] derive Encode/Decode for json, xml, yaml, toml, csv, protobuf, sql rows
- [ ] derive Equal, Hash, Compare, Clone, Default, Debug
- [ ] no run-time reflection, so no binary-size cost when unused

**Packages, lock file and capabilities (#146)**
- [ ] `tin.mod` with module path, version, dependencies; `tin.lock` with content hashes
- [ ] resolver: minimal version selection, replace directives, vendoring
- [ ] registry protocol, checksum database, offline mirror
- [ ] capabilities per package: net, fs, exec, env, time, unsafe; granted in the root module
- [ ] `internal` and visibility rules; import cycles diagnostics
- [ ] semantic import compatibility and editions

**Atomics**
- [ ] `atomic` package: Load, Store, Add, CAS, Swap on i32/i64/u32/u64/bool/pointer-free handles
- [ ] memory ordering defined (acquire/release/seq_cst) and mapped to arm64 and amd64
- [ ] cross-core counters for metrics; `once` for process-wide init
- [ ] no mutex by design: document the patterns that replace it (per-core state, relay messages)

**Iterators and range over functions**
- [ ] iterator protocol as a compile-time shape (`next` returning optional)
- [ ] `for x := range f` with early exit and defer semantics
- [ ] `seq`, `seq2` types; adapters: Map, Filter, Take, Zip, Chunk, Collect
- [ ] library `*Seq` forms: strings.SplitSeq/Lines, maps.Keys/Values, slices.Values/All/Collect/Sorted, bytes, regexp

#### 12.3 Library milestones (function-level)

**time (zones, layouts, timers)**
- [ ] `Time` with wall and monotonic readings, `Duration` formatting and parsing (in tide today)
- [ ] `Month` and `Weekday` enums with `String`
- [ ] `Location`, `LoadLocation` from the tz database embedded or read from the system, fixed zones
- [ ] layout-based `Format` and `Parse` with all Go layout verbs, plus RFC 3339 and HTTP dates (done)
- [ ] `Truncate`, `Round`, `AddDate`, `Sub`, `Before`, `After`, `Equal`, `Compare`, `UnixMilli` family
- [ ] `Timer`, `Ticker`, `After`, `AfterFunc`, `Sleep` on the task scheduler, with the ambient deadline
- [ ] `time.Now` via vDSO on Linux, `mach_absolute_time` on macOS
- [ ] verify against Go: layouts across 10,000 instants in 20 zones, DST edges, leap years

**regexp**
- [ ] parser for RE2 syntax with flags and Unicode classes (uses glyph tables)
- [ ] compiler to a Pike VM and a one-pass and a backtracking matcher selected like Go
- [ ] `MatchString`, `Find*`, `FindAll*`, `Submatch`, named groups, `ReplaceAll*`, `Split`, `Longest`, `Expand`
- [ ] DFA or lazy DFA for speed; literal prefix acceleration with memchr and SIMD
- [ ] linear-time guarantee tests and a ReDoS corpus
- [ ] verify against Go on the RE2 test corpus (several thousand patterns and inputs)

**encoding/json (beyond argo)**
- [ ] streaming `Decoder` and `Encoder` (`Token`, `More`, `Buffered`, `UseNumber`, `DisallowUnknownFields`)
- [ ] `RawMessage`, `Number`, custom (un)marshalers via shapes or derivation
- [ ] `MarshalIndent`, HTML escaping flag, `Valid`, `Compact`, `Indent`
- [ ] dynamic values: a `json.Value` enum for maps and arrays of unknown shape
- [ ] error messages with offsets and field paths as Go reports them
- [ ] verify against Go on the JSONTestSuite and a fuzz corpus

**strconv and fmt completion**
- [ ] `AppendInt`, `AppendFloat`, `AppendBool`, `AppendQuote*`, `QuoteToASCII`, `QuoteToGraphic`, `IsPrint`, `IsGraphic`, `CanBackquote`
- [ ] `ParseComplex` decision; `FormatFloat` with all formats and precisions verified against Go
- [ ] `say`: width and flags on `%q` (#149), `%+q`, `%#q`, `%x` on slices, `%b`, `%o`, `%O`, `%e`/`%G` corner cases, `%T`, `%p`, `%*d`, `%[1]d` argument indexes
- [ ] `Sscanf`, `Sscan`, `Fscan` family
- [ ] verify against Go over a generated corpus of formats and values

**os, io/fs and files**
- [ ] `File` type with Read, Write, Seek, ReadAt, WriteAt, Close, Sync, Truncate, Stat
- [ ] `Stat`, `Lstat`, `FileInfo`, `FileMode`, permissions, `Chmod`, `Chown`, `Chtimes`
- [ ] `MkdirAll`, `RemoveAll`, `ReadDir` with entries, `CreateTemp`, `MkdirTemp`, `Symlink`, `Readlink`, `Link`
- [ ] `io/fs` shapes (`FS`, `File`, `DirEntry`), `fs.WalkDir`, `embed.FS`, `testing/fstest`
- [ ] `os/exec`: spawn, pipes, wait, environment, context deadlines, no fork in multi-threaded runtime (posix_spawn)
- [ ] `os/signal`, `os/user`
- [ ] file I/O off the core on helper threads or io_uring, so handlers never block

**net and net/http**
- [ ] `net`: `Dial`, `Listen`, `Conn`, `Listener`, UDP, Unix sockets, `LookupHost`, `ParseIP`, `IPNet`, `JoinHostPort`, deadlines
- [ ] `net/netip`: `Addr`, `Prefix`, `AddrPort` with the parsing already done inside `link`
- [ ] `net/http` client: `Client`, `Transport` with pooling per core, redirects, cookies (`cookiejar`), timeouts, HTTP/2, proxies
- [ ] `net/http` server: HTTP/2, HTTP/3 (later), `ServeMux` patterns, middleware helpers, `ResponseController`, `Flusher`, `Hijacker` equivalents, graceful shutdown, `FileServer`
- [ ] `httptest`, `httputil` (`ReverseProxy`, `Dump*`), `pprof` endpoints
- [ ] `mime`, `mime/multipart`, `net/textproto`, `net/mail`, `net/smtp`
- [ ] WebSocket (done), Server-Sent Events, gRPC transport on HTTP/2
- [ ] conformance: h2spec, Autobahn for WebSocket, curl-based suites, differential tests against Go's server

**crypto and TLS (#124)**
- [ ] TLS 1.3 client, then server, then TLS 1.2; certificate verification with the system roots
- [ ] `x509` parsing and verification, PEM, `asn1`, PKCS#1/8, `pkix`
- [ ] hashes: streaming SHA-1/224/256/384/512/3, MD5, BLAKE2, HMAC, HKDF, PBKDF2, scrypt, argon2, bcrypt
- [ ] ciphers: AES (hardware), GCM, CTR, CBC, ChaCha20-Poly1305, XChaCha
- [ ] public key: RSA (keygen, PSS, OAEP, PKCS1v15), ECDSA, Ed25519, ECDH/X25519, P-256/384/521
- [ ] `crypto/rand` with `Reader`, `Int`, `Prime`; `subtle` complete; ML-KEM later
- [ ] constant-time tests and Wycheproof vectors
- [ ] hardware paths for arm64 (AES, SHA, PMULL) and amd64 (AES-NI, SHA-NI, PCLMUL, AVX2)

**database/sql shape**
- [ ] shared driver shape (on shapes): `Conn`, `Stmt`, `Rows`, `Tx`, `Result`, `Named` args
- [ ] `Open`, `Query`, `QueryRow`, `Exec`, `Prepare`, `BeginTx` with isolation, `Ping`, pool settings per core
- [ ] `Scan` into variables, structs (derivation) and `sql.Null*` types
- [ ] context deadlines and cancellation on every call
- [ ] drivers: mysql and postgres (done, pooled per core) moved onto the shape; sqlite (pure Tin), clickhouse, mssql
- [ ] verify against live servers in CI (MySQL 8, Postgres 16) with Go's `sqlmock`-style corpus

**compress, archive, encoding**
- [ ] `flate`, `gzip`, `zlib`, `lzw`, `bzip2` readers and writers; zstd and brotli as Tin-native extras
- [ ] `archive/tar` and `archive/zip` on the io shapes
- [ ] `encoding/*`: base32/64 variants, hex dump, csv, xml, gob decision, asn1, pem, binary (derivation), ascii85
- [ ] `unicode/utf16`, `utf8` completion, `text/template`, `html/template`, `text/tabwriter`, `text/scanner`, `go/*` decision (a Tin parser package instead)
- [ ] verify each codec against Go with round trips and cross-decoding

**math, big numbers, random**
- [ ] `math` special functions: Expm1, Asinh/Acosh/Atanh, Sincos, FMA, Nextafter, Remainder, Logb, Gamma, Lgamma, Erf, Erfc, Bessel family
- [ ] `math/big`: Int, Rat, Float with Karatsuba/Toom and assembly kernels; `math/cmplx` if complex is accepted
- [ ] `math/rand` and `rand/v2`: PCG, ChaCha8, Zipf, ExpFloat64, Source shape
- [ ] verify against Go bit for bit; keep the FMA note in docs/PERFORMANCE.md current

**runtime, sync, testing, misc**
- [ ] `runtime` equivalents: `NumCPU`, stats, `Gosched`-like yield, `debug.Stack`, build info
- [ ] `sync`-equivalents without mutexes: `once`, atomics, per-core maps, `relay` pools
- [ ] `testing`: `T`, `B`, `F`, subtests, `TempDir`, `Setenv`, `Cleanup`, `testing/quick`, `iotest`, `fstest`, `slogtest`
- [ ] `log` and `log/slog` parity in herald, `expvar`, `flag` completion, `html`, `image/*`, `hash/*` completion, `plugin` (not planned), `go/*` (not planned)

#### 12.4 Compiler performance work packages

**Inliner**
- [ ] inline multi-statement functions with a size and call-count model
- [ ] inline across packages after monomorphization
- [ ] inline `mut` receivers and methods on structs
- [ ] keep stack traces correct through inlined frames
- [ ] benchmarks: sort, math, strings, json before and after

**Intrinsics and SIMD**
- [ ] map bits, math and memory functions to single instructions on both CPUs
- [ ] vector types and operations in the language, with lane-width checks
- [ ] memchr, memcmp, utf8 validation, base64, hex, hashing kernels (NEON and AVX2)
- [ ] runtime CPU feature detection and function multiversioning

**Middle end**
- [ ] introduce SSA form with a verifier
- [ ] constant propagation, CSE, DCE, dead store elimination
- [ ] bounds-check elimination from loop facts
- [ ] loop invariant motion, unrolling, strength reduction
- [ ] register allocation and spill heuristics tested on the whole corpus

**Memory**
- [ ] allocation sinking and stack allocation of request-local values
- [ ] pool layout tuning: size classes, alignment, prefetch
- [ ] `keep` copy elision when the source is dead
- [ ] zero-cost iteration over maps (insertion-ordered) with cache-friendly layout

**Build speed**
- [ ] parse and check packages in parallel
- [ ] per-package object cache with content hashes
- [ ] incremental relink; `tin run` under a second for small programs
- [ ] compiler self-time profile in CI

#### 12.5 Quality gates every release must keep

- [ ] `make bootstrap` is a fixed point on darwin-arm64, linux-arm64 and linux-amd64
- [ ] every stdlib package has a Go twin test that is identical or a documented, tested difference
- [ ] every bug fix has a regression case tied to an issue
- [ ] no Python added under tools/ci beyond extending existing files; no Go in the build (Go stays only as a comparison baseline)
- [ ] docs regenerate cleanly (`gendoc.py`, `gen_unicode.py`, COVERAGE) and are checked in CI
- [ ] benchmarks do not regress by more than 2 percent without a written reason
