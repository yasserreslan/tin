# base

The foundation the native packages build on. It knows nothing about Tin; see
[docs/NATIVE.md](../docs/NATIVE.md) for the conventions every module follows.

| module | what it is |
|---|---|
| `util.h` | attribute macros, the out-of-memory policy (`oom`), checked size arithmetic, Go-worded `errno` text |
| `arena.h` | bump allocation for data that dies together; zeroed, 16-byte aligned |
| `str.h` | `Str`, a length-delimited byte string that may contain NULs |
| `buf.h` | `Buf`, a growable heap buffer with `printf`-style appends |
| `vec.h` | typed growable arrays stored in an arena (`VEC(T)`, `vec_push`) |
| `map.h` | a `Str` to pointer hash map stored in an arena |
| `diag.h` | source positions and diagnostics as data, rendered once by the driver |
| `fs.h` | whole-file reads and writes, temporary directories, with Go-worded errors |
| `proc.h` | running a child process (output captured or inherited), and running a function on a big stack |

Tests are in `tests/`, one program per module: `make native-test`.
