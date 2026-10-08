# crucible helpers (#920)

The testing helpers that sit beside `crucible` (labeled checks, `Done`, `Bench`, `T` and `B`). Each is a
package under `crucible/`, imported as `import "crucible/iotest"` and so on.

| package | Go's | what it does |
|---|---|---|
| `crucible/iotest` | `testing/iotest` | `TimeoutReader`, `HalfReader`, `DataErrReader` over the `io.Reader` shape; `TruncateWriter` over `io.Writer`. |
| `crucible/quick` | `testing/quick` | `Check`, `Check2`, `CheckEqual` over generators passed per parameter; `Int64`, `Uint64`, `Float64`, `Bool`, `Str`, `Bytes`. |
| `crucible/fstest` | `testing/fstest` | `MapFS` (an in-memory `fs.TreeFS`) and `TestFS`, which walks a file system with `fs.WalkDir` and checks it. |
| `crucible/slogtest` | `log/slog/slogtest` | `Check` runs records through `herald.Line` under a fixed clock and reports lines outside herald's layout. |
| `crucible/cryptotest` | `testing/cryptotest` (seeding), `crypto/subtle` | `Filled`, `Seeded`, `Flip` and `Equal` (`seal.Equal`) for constant-time comparison tests. |

## Design notes

- Helpers return a `fault` (or a list of problems, `slogtest.Check`) rather than taking a test handle, so a
  strict test decides how to report them with `crucible.True` and friends.
- Tin has no read that returns data and an error together, and a stream ends with a read of 0. So
  `DataErrReader` passes the data on and fails on the read after the source's end. `TruncateWriter` reports
  every byte as written, as Go's does.
- `quick` cannot ask a type for its generator at run time, so each check takes a generator per parameter.
  The generators are seeded by `Config.Seed` (default 1), so a run is the same on every run.
- `fstest.TestFS` collects every problem and fails with them joined. Directories come from the names of the
  entries, as in Go.
- `cryptotest` carries its seed in each buffer: there is no process-wide random source to seed.

## Known gaps

- `synctest` is not implemented. Go's bubble needs a fake clock that the scheduler honours. Tin has none:
  `tide.Now`, `tide.Wall` and `tide.Sleep`, and the timers behind `within`, `after` and deadlines, read the
  runtime's clocks directly. `herald.SetClock` fakes log timestamps only. Adding one would be a runtime
  change (toolchain/runtime), which this package does not make.
- `slogtest` has no Go twin: no Go package produces herald's layout. It is pinned by the strict test
  (`toolchain/tests/v2/crucible_helpers.tin`).
- `slogtest.Check` replaces herald's clock on the calling core for the rest of the program.
- `MapFS` cannot be the type of a struct field from another package: the compiler reports `undefined type`
  for a cross-package named map in a field (`mime.Params` in a struct field gives the same error). The
  strict test passes it as a type argument instead.
- `fstest.MapFS` has no `Glob` or `Sub` of its own; `fs.Glob` and `fs.Sub` work over it.
- `cryptotest.Flip` panics when its index is out of range, as any index does.

Twin check: `sh tools/ci/tin.sh crucible_check` (`bench/ref/crucible`, `tools/ci/fixtures/crucible.tin`).
