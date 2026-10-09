# Decision: no `plugin` package; extensions run as child processes (#924)

Status: **decided** by this document. It changes no runtime, compiler or library code and adds no test. It answers
the open question in design/roadmap.md (section 12, the `plugin` item) and fills the `plugin` row of
design/coverage.md with a pointer here.

## 1. Decision

1. **Tin has no `plugin` package and no dynamic loading.** A program does not open a shared object, does not
   resolve symbols at run time and does not call code that was not in the build. `plugin.Open` has no
   counterpart and will not get one.
2. **What a Tin program uses instead:**
   - **Code reuse happens at build time.** A dependency is a source package: imported by path, recorded in
     `tin.lock` with its content hash, copied into `vendor/` and checked for the capabilities its manifest
     declares (design/design_foundations.md section 8; toolchain/docs/PACKAGES.md).
   - **Code that must run beside a server, or that is supplied after the build, runs as a child process.** The
     parent starts it with `spawn` (toolchain/std/spawn) and talks to it over standard input and output with a
     versioned, length-bounded frame protocol. Section 3.2 gives the rules. This is the pattern `tin lsp` already
     uses (toolchain/docs/TOOLING.md section 3.4).
   - **Hot reload is not provided.** The answer to the open question in design/roadmap.md section 11 is fast
     rebuilds with a zero-downtime restart, not in-process patching.
3. **An ABI-stable extension package is rejected for now.** Tin promises no binary ABI between packages or
   between a package and the runtime (section 2.3). Adding one would be a language-level commitment that nothing
   in the tree needs yet. Section 8 says what would reopen it.

## 2. Why dynamic loading does not fit

### 2.1 The deployment model

Linux arm64 and Linux x86-64 are the production platforms, and both produce **static PIE executables**: no
`PT_INTERP`, no `PT_DYNAMIC`, no imports, no C library. The linker stops with E990 if a program would import a
function (toolchain/docs/PORTING.md section 2; toolchain/docs/COMPILER.md, the ELF writers; toolchain/docs/ERRORS.md,
E990). `tools/ci/static_check.tin` asserts this in CI. The binary runs on any distribution and on `FROM scratch`
(toolchain/docs/PORTING.md); the example image is `debian:bookworm-slim` (`examples/k8s/Dockerfile`).

A `dlopen` needs a dynamic loader and a C library, and the runtime has neither. The only way to load a shared
object would be an ELF loader written in Tin. That loader would be trusted code (the `unsafe` capability) running
machine code that nobody compiled with the program. That is a new trusted surface in every server, bought only to
undo a property the deployment depends on: one file that is the whole program. Go's own documentation makes the
same point for a deployment built from a single static executable: a plugin "may require careful configuration to
ensure that the various parts of the program be made available in the correct location in the file system (or
container image)".

### 2.2 Whole-program guarantees

Several checks in Tin are whole-program, and loaded code would sit outside all of them:

- **Capabilities.** The compiler builds the call graph of the whole program and rejects a call that reaches
  `net`, `files`, `spawn` or `unsafe` without a grant (`E804 CAPABILITY`, toolchain/docs/PACKAGES.md, "Capabilities";
  `tin caps`). Code loaded at run time is not in that graph.
- **Replay.** The replay design says a recording is complete "without an `external` keyword" because every effect
  reaches the operating system through a library function the compiler can see, and `rt_effect` records it
  (design/design_semantics.md section 12.1). Code that the compiler never saw can make effects the recording
  does not cover.
- **Region and `dyn` rules.** The `dyn` table layout is internal to a whole-program build (design/design_dyn.md),
  and region checks are whole-program (design/roadmap.md section 11, the hot-reload item).

The capability table already states the limit: a call through a function pointer is not visible to the walk, so
`exec` has no entry point yet (toolchain/docs/PACKAGES.md). Dynamic loading would add many such calls at once.

### 2.3 No ABI to load against

A plugin compiled against one Tin build would bind to layouts that Tin does not promise to keep:

- The runtime's task and tape layouts change between versions. Adding the tape word to a task (#241) moved `taskWords`
  to 57 (design/interface_replay.md section 2).
- Packages are source. "A dependency is source: there is no binary package format" (design/design_foundations.md
  section 8).
- The `dyn` table layout is internal to a whole-program build, and "no cross-package ABI is promised"
  (design/design_dyn.md).

Go's `plugin` documentation names the same failure mode: "Runtime crashes are likely to occur unless all parts of the
program (the application and all its plugins) are compiled using exactly the same version of the toolchain, the same
build tags, and the same values of certain flags". Tin cannot give that guarantee for code built later.

### 2.4 Isolation

A loaded object runs in the server's address space, on its cores, with its memory. A crash in it outside a task or a
guard, such as a stack overflow, ends the process. design/design_semantics.md section 1 classes that as process
failure, recovered by restart. A child process turns the same failure into an exit status the parent can read, and the
parent keeps serving.

### 2.5 macOS

`dlopen` works on macOS, which might suggest a macOS-only feature. AGENTS.md and toolchain/docs/PORTING.md say macOS
arm64 is for development only, and no design is justified by macOS behaviour. A plugin that works only on the
development platform is not a design.

### 2.6 Go's own warnings

`go doc plugin` (Go 1.27.1) gives four reasons that apply here: plugins are "supported only on Linux, FreeBSD, and
macOS, making them unsuitable for applications intended to be portable"; they are "poorly supported by the Go race
detector"; a bug in the host "could be exploited by an attacker to load dangerous or untrusted libraries"; and the
loaded package "cannot be closed". Go's documentation recommends "sockets, pipes, remote procedure call (RPC), shared
memory mappings, or file system operations" instead, which is the choice in section 3.2.

## 3. What a Tin program uses instead

### 3.1 Reuse at build time

Shared code is a source package. `import "github.com/ana/geo"` names it, `tin.lock` pins its hash, `tin vendor`
copies it, and its `tin.mod` declares what it may touch (toolchain/docs/PACKAGES.md). The reviewer sees the source and
the capabilities, which is the reason for packages in Tin (design/design_foundations.md section 8).

### 3.2 A child process behind a versioned protocol

When code must run beside a server, or be supplied after the build, it runs as a child process:

- **Start it with `spawn`.** `spawn.Start` takes an argument vector and never a shell. Give `Env` explicitly
  (`Env.Set` or `Env.Empty`) and `Dir` explicitly. Pipe `Stdin` and `Stdout`; send `Stderr` to `Null` or to a
  pipe the parent reads with a bound (toolchain/std/spawn).
- **Bound every read.** A frame carries its length. The reader checks the length against a maximum before it
  allocates, and fails the exchange past it. `spawn.Run` caps captured output at `MaxOutput`.
- **Version and reject.** The frame starts with a magic, a protocol version and a kind, and the reader rejects any
  other value. A change to a frame is a new version, never an edited one. The tit protocol does this with
  `"TITP"`, a version byte, a kind byte and a `u32` length, and a header at most 16 MiB (design/tit.md section 15).
- **Bound time.** Run the exchange under `within`. A deadline kills the child and reaps it, and the call fails with
  `fault.DeadlineExceeded` (toolchain/std/spawn, `Run` and `Process.Wait`).
- **Always reap.** Every `Start` is followed by `Wait`. A child never waited for stays a zombie (toolchain/std/spawn).
- **Declare the capability.** A package that starts processes declares `spawn` in its `tin.mod`. Without it,
  `E804 CAPABILITY` rejects the call (toolchain/docs/PACKAGES.md).
- **Record the exchange.** A request whose behaviour depends on a child's reply must record each exchange through
  `rt_effect` as a replay effect kind, or the request cannot be replayed (section 7). No such kind exists yet.

The pattern is already in the tree. `tin lsp` speaks the Language Server Protocol on standard input and output
and runs the compiler for each document through `spawn.Run` (toolchain/docs/TOOLING.md section 3.4;
`tools/lsp/exec_linux.tin`).

### 3.3 Hot reload

Development iteration is a rebuild and a restart, and production uses a zero-downtime restart (design/roadmap.md
section 3). The open question in design/roadmap.md section 11 is answered as "restart", and the work to make restarts
fast is compile speed (roadmap section 11) and graceful restart (roadmap section 3).

## 4. What is lost

- **In-process loading.** Code that is chosen from configuration and loaded into a running process without a restart
  has no counterpart. The replacement is a restart or a rolling deploy, which is slower and needs a new image or
  binary for each change.
- **In-process call cost and shared data.** A child call is a round trip over a pipe with a copy of each frame, not a
  call and not a shared structure. No cost is claimed here. Any performance statement needs Linux numbers measured on
  the same machine in the same run (AGENTS.md, "Benchmarks are Linux"; toolchain/docs/PERFORMANCE.md).
- **Closed-source binary extensions in the build.** Packages are source (section 2.3). A binary extension can still be a
  separate executable that the protocol drives, but nothing links it into the program.
- **Plugin initialization at a later time.** Packages initialize at program start, and that stays true.
- **Unloading.** Nothing is unloaded, because nothing is loaded.

## 5. What is kept

- **Static executables.** One file per program, no loader, `FROM scratch` images, no C library.
- **Whole-program checks.** Capabilities (`E804`), region checks and the replay design all keep their guarantees,
  because every line of code is in the build.
- **Crash isolation for extensions that run as processes.** A child that crashes or hangs is killed and reaped at its
  deadline, and the server keeps serving. The child gets its own memory and its own OS limits.
- **Reuse with review.** Shared code is source, hashed and capability-declared.
- **A protocol with an upgrade path.** A version byte and a new path for a new version (design/tit.md section 15)
  let parent and child be upgraded separately.

**Not provided, and not claimed:** a sandbox. A child runs with the same user privileges as its parent. Tin programs
have no sandbox API: the runtime and the library have no seccomp, namespace or chroot support. The CI tools use
`unshare -rn` and `chroot` to test offline builds and the empty-root static run (`tools/ci/packages_check.tin`,
`tools/ci/static_check.tin`); that is test harness, not a program facility. The other "sandbox" mention is a future
browser playground (design/roadmap.md, the playground item). Isolation here means crashes and memory, not privilege.

## 6. What already exists in the tree

| piece | where | what it gives a plugin replacement | what it does not give |
|---|---|---|---|
| `spawn` | toolchain/std/spawn | start a child with pipes, deadlines, output cap, kill and reap; Linux and macOS | no replay record; no sandbox |
| `tin lsp` | toolchain/docs/TOOLING.md section 3.4, `tools/lsp` | a shipped child-process protocol over stdio | not a general extension mechanism |
| TITP frames | design/tit.md section 15; `ReadFrame` in products/tit/transport | a versioned, length-bounded frame layout to copy | `ReadFrame` decodes a whole body; nothing in toolchain/std frames a byte stream for a child |
| capabilities | toolchain/docs/PACKAGES.md | `spawn` and `E804` for the calling package | `exec` has no entry point yet |
| replay effects | design/design_semantics.md section 12.1, `rt_effect` | the hook for recording an exchange | no kind for child processes |
| replay capsules | design/interface_replay.md | records a request's effects to rerun it | not an extension or plugin protocol; they do not run third-party code |

Nothing in the tree is a sandbox or an ABI-stable extension package. A replay capsule is a recording, not a way to load
code: it runs a request again with its recorded effects served from the file.

## 7. Gaps this decision leaves

These are follow-ups. None is done here, and each needs its own issue or design note.

1. **Child processes are not replay effects.** `toolchain/std/spawn` calls no `rt_effect`, and the replay `Kinds`
   list has no process kind (toolchain/docs/STDLIB.md, replay). A request that depends on a child cannot be replayed
   until a kind (for example `spawn.call@1`) records each exchange. A new kind needs approval under
   design/interface_replay.md.
2. **`exec` has no entry point**, and calls through function pointers are invisible to the capability walk
   (toolchain/docs/PACKAGES.md). Any future entry point must stay visible to the walk or be marked.
3. **No sandbox.** A child has the parent's privileges. A sandbox is a separate decision.
4. **C libraries** are a different question. The FFI policy (design/roadmap.md section 11) decides whether user code may
   call C, and a shared object loaded from C would fall under that decision, not this one.

## 8. When this decision reopens

Reopen it only with a concrete case, measured on Linux (AGENTS.md): a child-process protocol that costs too much for a
real workload, or a need to run code that cannot be built from source in the program. The answer would then be a
design note for an ABI with versions, a capability for loading, and a replay story, not a `plugin` package that
copies Go's API.

## 9. Cross-references

- design/coverage.md, the `plugin` row, the source of toolchain/docs/COVERAGE.md.
- design/roadmap.md, section 3 (graceful restart), section 11 (hot reload and FFI open questions), and the `plugin` item
  in section 12.
- design/design_foundations.md section 8 (packages are source, capabilities).
- design/design_dyn.md (the `dyn` table is internal to a whole-program build).
- design/design_semantics.md sections 1 (process failure) and 12 (replay, effects).
- design/interface_replay.md (task layout, effect kinds, the replay reader).
- design/tit.md section 15 (the TITP frame).
- toolchain/docs/PACKAGES.md (capabilities, `E804`), toolchain/docs/TOOLING.md section 3.4 (`tin lsp`), toolchain/docs/PORTING.md
  (platform roles and the static binary), toolchain/docs/COMPILER.md (the ELF writers), toolchain/docs/ERRORS.md (E990),
  toolchain/docs/STDLIB.md (`spawn`, `replay`), toolchain/docs/PERFORMANCE.md (benchmark policy).
- AGENTS.md (Linux is production, macOS is development; benchmarks are Linux).
