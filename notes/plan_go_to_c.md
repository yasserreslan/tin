# Go to C: what was decided, what is left

Written 2026-10-02. The aim is a tree where the tools around Tin are fast to build and run, and
where the Go that remains is there on purpose: as a comparison, never as infrastructure.

## Decided

- **Stage 0 is retired, not ported.** `bootstrap/` was the original Go compiler (`tinc0`, 1,656
  lines) for Tin's first syntax. A C port was written and was byte-identical to it on all 28
  legacy tests and error cases. But since the compiler's own sources moved to the new syntax,
  neither version can compile `selfhost/` any more (`selfhost/lex.tin:545: expected 'fn', found
  name 'func'`), so stage 0 was no longer a bootstrap path. Its one remaining job was running
  the legacy tests, and the self-hosted `tinc` can do that. The seeds in `seed/` are the root of
  the build; `make bootstrap` proves the fixed point. The legacy suite now runs against `tinc`
  only: `tools/ci/legacy_suite.py`.
- **The C that exists is a foundation, in packages.** `base/` (arena, strings, buffers, vectors,
  maps, diagnostics, files, processes) and `testkit/` are tested under sanitizers and built by
  rules that discover any new package from its `package.mk`. See docs/NATIVE.md.

## What Go is left (4,431 lines, 35 files)

| code | lines | decision |
|---|---|---|
| `bench/http/conformance` (the 26 HTTP edge-case checks) | 1,009 | port to C: it is run by CI (`tools/ci/http_check.py`), so it is the one Go program still on the critical path |
| `tests/graceful/graceful.go` | 244 | port to C: also run by `http_check.py` |
| `bench/http/hammer` (load generator) | 181 | port to C: the load generator is part of every performance number, and a C one can run closer to the hardware |
| `bench/ref/*` (stdlib checked against Go's library) | 2,098 | keep: Go's library is the reference |
| `bench/v2/*.go`, `bench/*.go`, `examples/demo_go` | 628 | keep: the comparison. A `clang -O2` column is worth adding next to Go |
| `bench/http/fast`, `gonet`, `bench/router/go`, `bench/v04/go` | 271 | keep: fasthttp, net/http and chi are the baselines |

Once the first three are ported, CI no longer needs the Go toolchain for anything but the
benchmark workflow, and `setup-go` leaves `ci.yml`.

## Next

1. Load generator (`hammer`) in C on top of `base`, using epoll/kqueue.
2. HTTP conformance client and the graceful-shutdown test in C; drop Go from `http_check.py`.
3. C baselines for `bench/v2` and a runner that reports Tin against both Go and `clang -O2`.
4. Performance work on the compiler, measured by those benchmarks (docs/PERFORMANCE.md section
   3): division by constants, constant materialization, length-fact bounds-check elimination,
   register allocation weights, alias information.
