# Native packages: C in this repository

Most of Tin is written in Tin: the compiler, the runtime and the standard library. The C in this
tree is for what has to exist before Tin does, or beside it: tools that measure and exercise
Tin programs (load generators, conformance clients), and the shared foundation they stand on.
It is built with the host's C compiler, with no dependencies beyond libc and POSIX.

This page is the contract for that code. Read it before adding to it.

## 1. Layout

Native code is organized by what it is, in packages. A package is a top-level directory with
this shape, and nothing else is special:

```
<pkg>/
  package.mk            declares the package: <pkg>_DEPS and <pkg>_PROGRAMS
  include/<pkg>/*.h     public headers, included as "<pkg>/name.h"
  src/*.c               the library, built into bin/native/lib<pkg>.a
  cmd/<prog>/*.c        one program per directory, linked with the package and its dependencies
  tests/*_test.c        one test program per file, linked with testkit
  README.md             what the package is for, its modules, its rules
```

| package | role |
|---|---|
| `base/` | the foundation every other package builds on: arena, strings, buffers, vectors, maps, diagnostics, files, processes |
| `testkit/` | the unit-test framework used by every package's `tests/` |

A package may depend on `base` and on packages below it; dependencies are declared in
`package.mk` and must not form a cycle. `base` depends on nothing in the repository and knows
nothing about Tin.

### Adding a package

1. Create `<pkg>/` with `include/<pkg>/`, `src/`, `tests/` (and `cmd/<prog>/` for a program).
2. Write `<pkg>/package.mk`:
   ```make
   # loadgen: the HTTP load generator.
   loadgen_DEPS := base
   loadgen_PROGRAMS := hammer
   ```
3. Add a `README.md`. Run `make native-test`.

The Makefile, the build rules (`tools/mk/native.mk`) and CI discover the package by itself.

## 2. Building and testing

| command | what it does |
|---|---|
| `make native` | builds every library and program; programs land in `bin/` |
| `make native-test` | builds and runs every `*_test.c`; each is a program printing `ok` or `FAIL` |
| `make native-asan` | the same under AddressSanitizer and UBSan, in `bin/native-asan` |
| `make native-format` | applies `.clang-format` to every native source |
| `make native-clean` | removes the native build output |

`NATIVE_WERROR=-Werror` turns warnings into errors (CI sets it). `CC` selects the compiler.
Objects are built per file with dependency tracking, so a rebuild recompiles only what changed;
`make -j` builds in parallel.

CI runs `native-test` and `native-asan` with `-Werror` on macOS arm64, Linux arm64 and Linux
x86-64. Both must be clean, with no suppressions.

## 3. Conventions

**Language.** C11 plus POSIX.1-2008, as `native.mk` selects it. No compiler extensions except
the attribute macros in `base/util.h`. The code must compile warning-free with the flags in
`native.mk` on clang and gcc.

**Naming.** Functions are `module_verb` (`arena_alloc`, `buf_printf`, `lexer_lex`). Types are
`CamelCase` (`Arena`, `Buf`, `Diag`). Macros and enum constants are `UPPER_CASE`. Public names
carry their module's name, so a symbol says where it lives. Header guards are `PKG_NAME_H`.

**Headers.** A header is the module's contract: every public function says what it does with
ownership, what it returns on failure and what it does not promise. Include what you use, in
three groups (system, `base/...` and other packages, the package's own), each sorted.

**No globals.** State lives in a context value that is passed in. A package must be usable twice
in one process; the unit tests do exactly that.

**Memory.** Two kinds of storage, chosen by lifetime:
- *Arena* for data that all dies together (a compiler's tokens, tree and symbols): allocate
  freely, release with one `arena_free`. Arena memory is zeroed.
- *Buf* (and plain `malloc`) for text and data with its own lifetime. Whoever creates it frees it.

Out of memory is not an error value: `oom()` prints and aborts, so a tool that runs out of memory
crashes loudly and is never mistaken for one that rejected its input. Size arithmetic that can
overflow goes through `size_add` and `size_mul`.

**Errors.** A function that can fail returns `bool` (or a status) and reports the reason as
data: diagnostics go to a `Diag`, system errors become text in a `Buf` worded as Go worded them
(`open x: no such file or directory`). Passes never print; the driver renders once. Anything
that cannot happen is `assert`ed, never silently handled.

**Untrusted input.** Treat every input as hostile: lengths are checked before use, recursion is
bounded by a limit that is part of the contract (see `max_depth` in a compiler's context), and
nothing indexes past what it was given.

**Comments.** Say why, and state invariants and units. Do not narrate the code.

**Tests.** Every module has a `tests/<module>_test.c`. Test the contract, the edges and the
failures: empty input, a NUL in the middle, the largest and smallest values, a path that does not
exist. A bug fix adds the case that failed first.

## 4. Where things go

- A helper two packages need moves to `base` (with tests) rather than being copied.
- Anything specific to Tin's syntax or semantics lives in a package that is about Tin, not in
  `base`.
- Programs are thin: `cmd/<prog>/main.c` parses arguments and calls the library. Logic worth
  testing lives in `src/`.
