# Design: the standard-library packages Tin does not port (#932)

Status: decision record, 2026-10-09. For each Go package the milestone "Stdlib breadth 2" does not port, this
names Tin's counterpart or records why none is needed. `design/coverage.md` (published as
`toolchain/docs/COVERAGE.md`) links here for every package it marks `n/a` or `design`. No code is changed by
this record. Each package is checked against the tree at `origin/main`.

## How to read the tables

- **Counterpart**: a Tin package or construct that does the job for Tin programs.
- **n/a**: Tin needs nothing for this. The reason is in the last column.
- **#915**: the user-visible part belongs to the net/http issue (#915): an HTTP endpoint, a client hook, or a
  profile served over anvil.
- **Gap**: a need the counterpart does not cover. It is named, not decided here, so it is not dropped silently.
- **koussa**: the files in Anghami's service that import the package, production and test, from
  `design/coverage.md`. It is a yardstick, not a port target. `0` means no file imports it.

## go/*

| Go package | koussa | Tin counterpart | Decision and reason |
|---|---|---|---|
| go/ast | 0 | declarations from `tinsym` (`Sym`, `Member`) | n/a. The syntax tree stays inside the compiler (`toolchain/compiler/`). Tools get declarations from `tinsym.Parse`; see "Does tinsym publish a parser?" below. |
| go/build | 0 | `tin build`, `tin.mod` (PACKAGES.md) | n/a. Building is the compiler's and the package manager's job (#146); no library reads build lists. |
| go/build/constraint | 0 | `NAME_linux.tin`, `NAME_darwin.tin` file names (PORTING.md) | n/a. Platform choice is by file name, made by the compiler. There are no `//go:build` expressions to parse. |
| go/constant | 0 | exact constant arithmetic in the checker (E223) | n/a. The compiler evaluates untyped constants exactly; no program evaluates them at run time. |
| go/doc | 0 | `tools/gen/gendoc.tin` writes STDLIB.md from the comments; `tin doc` is roadmap | n/a for now. Docs come from the one-line comments the rules require. A rendered doc tree is the package registry item in `design/roadmap.md`, not a port. |
| go/doc/comment | 0 | none | n/a. Doc comments are plain text; nothing parses their markup. |
| go/format | 0 | `packages/tinfmt` (`Format`), command `tin fmt` | Counterpart. `tinfmt.Format` returns the canonical whitespace of a source file. |
| go/importer | 0 | the compiler's loader (`import "NAME"`, `import "./dir"`) | n/a. The compiler loads packages from source; no export data is read. |
| go/parser | 0 | the compiler's parser (`toolchain/compiler/parse.tin`) | n/a. There is one parser, the compiler's. |
| go/printer | 0 | `packages/tinfmt` | Counterpart for the output. `tinfmt` changes whitespace and keeps every other byte, so no tree printer is needed. |
| go/scanner | 0 | `toolchain/std/scan` (Go's text/scanner over Go's token rules); `tinsym.Keywords` | n/a for Tin source. `scan` tokenizes Go-style text. Tin's keywords are listed in `tinsym.Keywords`, and the lexical rules are the compiler's. |
| go/token | 0 | `scan.Position` (1-based lines and columns); `tinsym` (0-based line, byte column) | n/a as a package. Each package that needs a position keeps its own plain value. |
| go/types | 0 | the compiler's diagnostics (ERRORS.md); `tinsym.Resolve`, `tinsym.Complete` | n/a. No type checker is published. Names and declarations are; the types of values are not (`tinsym.Complete` says so). |
| go/version | 0 | `tin version`; every program is edition 1 (TOOLING.md) | n/a. There is one language edition, so no version comparison is needed. |

### Does tinsym publish a parser?

**No.** `tinsym` publishes what the editor needs and no tree. It exposes `Parse`, which reads the JSON lines of
`tinc -symbols -json` (the declarations and the diagnostics, protocol 1, TOOLING.md section 3.3); `Resolve` and
`Complete`, which answer name questions from those declarations; `Keywords`; and text helpers (`WordAt`,
`LineOf`, `ImportsOf`, `CurrentPkg`, `DirOf`, `TypeParts`) that read the text, not a tree. The reasons for not
publishing a lexer or a parse API:

1. There is one grammar. The compiler is the only parser, and the diagnostics and the language server must agree
   with it. A second parser in a library would drift from it at every change to the language.
2. No consumer in this milestone needs a tree. `tin lsp` takes what it needs from `-symbols`.
3. A published tree is a compatibility promise for the grammar. If a second consumer asks, the right step is to
   export the compiler's AST as a new issue, not to copy the parser into a package.

## sync and sync/atomic

| Go API | koussa | Tin counterpart | Decision and reason |
|---|---|---|---|
| sync.Mutex, sync.RWMutex | 69+32 (sync) | none | n/a. A task on a core switches only at a wait (RUNTIME.md, request tasks), so a section that does not wait is exclusive. Data shared across cores is a read-only `shared let` or an atomic (LANGUAGE.md section 11). No lock is needed. |
| sync.WaitGroup | | `scope` with `s.spawn`, and `parallel { }` (LANGUAGE.md section 11) | Counterpart. The scope waits for every child, and the first fault cancels the rest. |
| sync.Once, OnceFunc, OnceValue | | `once { }` per core; `on app.start` (runs once before `main`); `shared let x = build()` (built on core 0 before the cores start) | Counterpart. `once { }` runs the first time each core reaches it. A process-wide once is `on app.start` or a `shared let`. |
| sync.Cond | | `select` over a `lane` or a `relay` message | n/a. A wait is a message: `select` waits for the value the condition would have announced. |
| sync.Pool | | none | n/a. There is no collector. A request's memory goes back to the core's request pool when the request ends (RUNTIME.md), and what outlives a request is `keep`ed. A reuse pool for long-lived objects is added only with a measured case. |
| sync.Map | | a per-core `map` global for data one core owns; a `shared let` for read-only data; `atomic` for counters | n/a for now, as the issue says: a sync.Map-style helper only when a real use shows up. |
| sync/atomic | 7+12 | `toolchain/std/atomic`: `Int` and `Bool` with Load, Store, Add, Swap, CompareSwap, used as `shared let hits = atomic.NewInt(0)` | Counterpart. Gaps: the unsigned types, and the function forms (`AddInt64` and so on); see Gaps. |

## io/ioutil

koussa 0. The package is deprecated in Go and is a set of shims over `os` and `io`, so it is not a package in Tin.
Each function maps as follows.

| ioutil function | Tin counterpart | Decision and reason |
|---|---|---|
| ReadAll | `io.ReadAll` | Counterpart. |
| ReadFile | `quarry.ReadFile` (at most 64 MiB; `quarry.ReadFileBound` sets another bound) | Counterpart. |
| WriteFile | `quarry.WriteFile` | Counterpart. |
| ReadDir | `quarry.ReadDir` (names) or `quarry.ReadDirEntries` (entries); `quarry.Stat` for the info | Counterpart. Go returns FileInfo values; Tin returns names, and a caller asks for the info it needs. |
| TempDir | `quarry.TempDir` (the directory, $TMPDIR or /tmp) | Counterpart for the directory. Creating one is a gap (see Gaps). |
| TempFile | none | Gap: quarry has no file-creating counterpart of Go's `CreateTemp`. |
| NopCloser | none | n/a. A reader that closes nothing is a few lines of the program's own type. |
| Discard | `io.Discard()` | Counterpart. |

## iter

| Go API | koussa | Tin counterpart | Decision and reason |
|---|---|---|---|
| iter.Seq, iter.Seq2 | 0 | slices returned by the package (`twine.Lines`, `atlas.Keys`, `atlas.Values`); `for x in` over slices, strings, maps and integer ranges; a `fn(yield fn(T) bool)` the program writes | n/a. The language has no range over functions (COVERAGE.md). A slice is the Tin iterator: it is bounded and needs no closure. A lazy sequence is written by hand. Range over functions is a language change, made only for a real use (none in koussa). |
| iter.Pull, iter.Pull2 | 0 | a task that sends its values on a `lane` | n/a. Pull needs a coroutine. A producer that must pause is a task that sends on a `lane`, and the consumer receives. |

## unique

| Go API | koussa | Tin counterpart | Decision and reason |
|---|---|---|---|
| unique.Make, unique.Handle | 0 | none | n/a. Interning gives handles that compare by pointer and share one copy of a value. Tin's str and map keys compare by value, and `same(a, b)` checks identity when a program asks. With no collector, an intern table would never free its entries, and no file in koussa needs a shared copy. |

## weak

| Go API | koussa | Tin counterpart | Decision and reason |
|---|---|---|---|
| weak.Make, weak.Pointer | 0 | `policy.NewCache` (at most a count of entries; a full cache drops its oldest entry) and `policy.Cached` (a time to live) | n/a. Weak pointers let a collector clear a cache entry. Tin has no collector, so caches are bounded by count and by age instead. |

## structs

| Go API | koussa | Tin counterpart | Decision and reason |
|---|---|---|---|
| structs.HostLayout | 0 | value structs (`type P value struct`, #631; design/design_layouts.md) | n/a. HostLayout promises a C-compatible layout. Tin's layouts are the compiler's (RUNTIME.md section 1). Foreign layouts wait for the FFI policy, which is an open question (design/roadmap.md, `runtime/cgo`). |

## unsafe

| Go API | koussa | Tin counterpart | Decision and reason |
|---|---|---|---|
| unsafe.Pointer, pointer arithmetic | 342+0 | raw operations in trusted code: `cast`, `load8`, `store8`, `__ld`, `__st` (LANGUAGE.md section 18) | n/a for programs. The operations are allowed in `toolchain/std/`, `packages/`, `toolchain/runtime/`, and in a vendored package whose `tin.mod` declares `caps unsafe` (PACKAGES.md). |
| unsafe.String, StringData, Slice, SliceData | | `str(b)` and `[]u8(s)`, which copy (LANGUAGE.md); `rt_str_from_raw` in trusted code | Counterpart for the copy. The zero-copy view is trusted code only. |
| unsafe.Sizeof, Alignof, Offsetof | | none | n/a. Sizes and offsets are the compiler's (RUNTIME.md section 1); the library's own struct layouts are checked against the kernel's (design/linux_abi.md). A program does not need them. |
| the package itself | | none | n/a as a package. The escape is the `unsafe` capability, not an import. An audited raw-memory package is roadmap work and not part of this milestone. |

The 342 files in koussa that import `unsafe` are not classified here. Each call site is decided when its file is
migrated.

## syscall

| Go API | koussa | Tin counterpart | Decision and reason |
|---|---|---|---|
| syscall.Signal | 2+1 | `signal.Signal`, with String | Counterpart. |
| syscall.Errno | | `fault`; errno messages are Tin code (RUNTIME.md) | Counterpart. A failed system call is a fault with the kernel's message. |
| syscall.Stat, Lstat | | `quarry.Stat`, `quarry.Lstat` | Counterpart. |
| syscall.Getpid, Getenv | | `quarry.Pid`, `quarry.Getenv` | Counterpart. |
| syscall.Kill, signalling a child | | `spawn`: `Process` signal and kill | Counterpart. |
| syscall.Exec | | none exported; `spawn` starts children | n/a. No public function replaces the process. The child is cloned and execs inside `spawn`. |
| syscall.Socket, Connect, Bind, Listen, Accept | | `wire`, `anvil`, `websocket` (capability `net`) | Counterpart. |
| syscall.Termios, ioctl | | `tty` | Counterpart. |
| syscall.Syscall, RawSyscall | | none | n/a. System calls are the runtime's `rt_sys_*` leaves, and a program cannot call `rt_` functions (E802). The Linux ABI is checked by `tools/ci/syscall_check.tin`. |

## runtime/*

The runtime's own instruments and their checks are listed after the table.

| Go package or API | koussa | Tin counterpart | Decision and reason |
|---|---|---|---|
| runtime: NumCPU, GOMAXPROCS | 2+2 | `hearth.Cores()`, `hearth.ID()`, `hearth.Run(n, entry)` | Counterpart. The cores are threads, fixed when the program starts. There is no setter. |
| runtime.Gosched | | `s.yield()` in a scope (LANGUAGE.md section 11) | Counterpart. |
| runtime.GC, SetFinalizer, KeepAlive | | none | n/a. There is no collector and no finalizer. `use` closes a resource when its function returns or its core stops (LANGUAGE.md, `use`, `on`, `once`). |
| runtime.NumGoroutine | | none | Gap, then #915: a count of tasks is an instrument. |
| runtime.Caller, Stack | | `fault.Backtrace(err)` and the panic's backtrace (RUNTIME.md section 7) | Counterpart for a fault or a panic. The stack of a running task is an instrument: #915. |
| runtime.ReadMemStats | | `hearth.HeapStats()` (a core's slabs and mappings), `hearth.RcStats()` (counted blocks and the limbo) | Counterpart, per core. A process total is the sum over the cores. |
| runtime/cgo | 0 | none | n/a. There is no cgo. Foreign calls are the FFI policy's open question (design/roadmap.md). |
| runtime/coverage | 0 | none | n/a for the runtime. Coverage reports for `tin test` are roadmap work in `crucible`. |
| runtime/debug | 3+0 | `fault.Backtrace` (Stack, PrintStack as for runtime.Stack); `hearth.MemLimit()` (read only) | Split. SetGCPercent and FreeOSMemory: n/a, there is no GC. SetMemoryLimit: the limit is the cgroup's and is read only. ReadBuildInfo: gap, since binaries embed no build info yet (roadmap). |
| runtime/metrics | 0 | `hearth.HeapStats`, `hearth.RcStats`, and `expvar` (#923) for published values | Partial counterpart. A named list of runtime metrics served over anvil is #915. |
| runtime/pprof | 0 | the `-g` DWARF line table and the `.symtab` read by `perf` (TOOLING.md section 8.2, "Profiling a Linux service") | Gap for an in-process pprof writer (roadmap, profilers). A profile served on an HTTP route is #915. |
| runtime/race | 0 | none | n/a. There is no shared mutable state between cores. Globals are per core; the shared values are read-only `shared let` or atomics. Tests on one core cannot race by construction. |
| runtime/trace | 0 | `replay` records one request's effects into a sealed capsule (#241, #242; `tin replay`) | Not a counterpart: a capsule is per request, not an execution trace. Per-core task traces are a gap (roadmap, profilers). Serving them over HTTP is #915. |
| testing/synctest | 0 | none | n/a for now. Time is the runtime's clock (`tide.Now`, recorded for replay), and there is no virtual clock. Deterministic tests come from exact output order and `crucible`. A virtual clock would be a runtime change, not a package. |

The instruments that exist, and the checks that guard them:

- `hearth` heap statistics: `tools/ci/heap_check.tin` (#345): size classes, slabs, mappings.
- Memory primitives: `tools/ci/memory_check.tin` (#178): cross-core returns and deterministic allocation failure.
- Threads: `tools/ci/thread_check.tin`: returned cores, reaped stacks, inherited masks, child faults.
- `shared let`: `tools/ci/shared_check.tin` (#344): a table is built once, not once per core, so resident memory does not grow with the cores.
- Signals and waits: `tools/ci/signal_check.tin`: process dispositions, task waits, per-core listeners.
- Linux ABI: `tools/ci/syscall_check.tin`: kernel errors, mappings, poll, stat, directory entries.
- Debug info: `tools/ci/test_dwarf.tin` reads the DWARF sections back for the three targets.

## net/http (top level)

| Go package | koussa | Tin counterpart | Decision and reason |
|---|---|---|---|
| net/http | 553+126 | `anvil` (server, Router, HTTPS, HTTP/2), `wire` (client), `websocket`, `tls` | Counterpart. Not ported as a package. Nothing beyond #915. |
| net/http/httputil (DumpRequest, DumpResponse) | | `dump` (#739) | Counterpart. |
| net/http/httptrace | 0 | `packages/httptrace` (client hooks, partial: DNS and connect are not observable) | Counterpart in progress, owned by #915. |
| net/http/pprof, net/http/cgi, net/http/fcgi | 1+0 (pprof) | none yet | #915. |

## Gaps

These are needs the counterparts do not cover. Each is named here so it is not silent. None is decided in this record.

- `quarry`: creating temporary directories and files (`MkdirTemp` and `CreateTemp`, the replacements for
  `ioutil.TempDir` and `ioutil.TempFile`).
- `atomic`: the unsigned types and the function forms (roadmap, `atomic` package: I64, U64, Bool).
- `hearth` and the runtime: a task count, a live stack, `ReadBuildInfo`, per-core execution traces, and an
  in-process pprof writer. Exposure over HTTP is #915.
- The migration of the koussa files that import `unsafe` (342 of them), one call site at a time.

## Roadmap lines this record decides differently

`design/roadmap.md` is not edited by this record. These lines say something else, and this record decides them:

- `unique`: the roadmap's next step, a per-core interning table, is dropped (n/a above).
- `iter`: the roadmap's `*Seq` iterator forms for `twine` and `atlas` become slices (above), unless a real use
  shows up.
- `sync`: the roadmap says WaitGroup, Once, Pool and Map "need routine-level equivalents". WaitGroup and Once
  have counterparts (`scope`, `once`); Pool and Map are n/a.
