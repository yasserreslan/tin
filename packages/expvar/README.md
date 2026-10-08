# expvar

Published variables a running program can read back, as Go's `expvar` does (#923). The
registry is process-wide: a variable made on one core is the same on every core.

## API

Constructors (each registers a name and panics if the name is taken):

- `NewInt(name) Int`, `NewFloat(name) Float`, `NewString(name) String`, `NewBool(name) Bool`,
  `NewMap(name) Map`. `NewBool` is an addition: Go 1.27 has no `Bool` (it publishes a flag with
  `expvar.Func`).

Variables:

- `Int`: `Add(delta)` (wraps on overflow, as Go's atomic add), `Set`, `Value`, `String`.
- `Float`: `Add(delta)` (compare-and-swap, so concurrent adds are not lost), `Set`, `Value`, `String`.
- `String`: `Set(value)` (copied), `Value`, `String` (quoted the way Go writes it).
- `Bool`: `Set`, `Value`, `String` (`true` or `false`).
- `Map`: `Add(key, delta)`, `AddFloat(key, delta)`, `Set(key, v dyn Var)` (shares `v`),
  `SetString(key, value)`, `SetBool(key, value)` (Tin helpers: Go makes map values with `new`, which
  has no handle here), `Get(key) ?dyn Var`, `Delete(key)`, `Init() Map`, `Do(f)`, `String`.
  `Add` and `AddFloat` on a key that holds another kind leave it alone, as in Go.

Registry:

- `Get(name) ?dyn Var`: the variable published as `name`, or nil.
- `Do(f fn(KeyValue))`: every published variable, in name order.
- `Handler(q, w)`: serves the registry as Go's `expvar.Handler` does: `application/json;
  charset=utf-8`, `{`, one `"name": value` line per variable in name order, `}`. A map's value is one
  line, `{"k": v, ...}`. Register it with `r.Get("/debug/vars", expvar.Handler)`.

`Var` is a shape with `String()` and a package-private method, so only this package's types implement
it.

## Design

- The registry is raw node memory (a list of entries, each pointing at a variable node) behind a
  spin lock. The lock is an `atomic.Int` in a `shared let`, and `__yield` spins while another core
  holds it. Nothing done under the lock waits.
- Int and Bool are one atomic word. Float is the bits of an `f64` in one atomic word, updated by
  compare-and-swap. A String's bytes are a `malloc` copy, replaced under the lock and freed after the
  swap; readers copy under the lock.
- Strings are rendered as Go's `appendJSONQuote` does: `\"`, `\\`, `\n`, `\r`, `\t`, `\uXXXX` for
  other control characters, `<`, `>`, `&`, U+2028 and U+2029, and U+FFFD for invalid UTF-8.
- Floats use the shortest `%g` form (`1e+21`, `1.234567e+06`, `1e-05`, `+Inf`, `NaN`).
- `Do` and `Handler` snapshot the entries under the lock and sort them after it; user callbacks run
  without the lock.

## Known gaps

- No collector. `Map.Delete` and `Map.Init` free their list entries, but a variable a handle may still
  use is never freed. Each deleted key therefore leaves one variable node (48 bytes) and its name.
- `cmdline` and `memstats` are not published (they describe the process, not the program).
- No `Func` and no `Publish`: a value is one of this package's types.
- A zero handle (`Int{}`) panics on use: make variables with the constructors or in a `Map`.
- Variables made in a request are process-wide, so a long-running server that creates a new name per
  request grows the registry without bound; create names once.

## Tests

- `toolchain/tests/v2/expvar.tin` (+ `.out`): every function, Go's float forms, the string escapes,
  wrap, map order and `Init`/`Delete`, `Get`/`Do`, the duplicate-name panic, and `Handler` through an
  anvil router.
- `sh tools/ci/tin.sh expvar_check`: the anvil server's `/debug/vars` against Go's `expvar.Handler`
  for the same registrations, after 200 requests.
