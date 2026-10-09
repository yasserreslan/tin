# sandbox

Runs a program fenced by Linux (#1002): its own cgroup v2, new namespaces, a root made of read-only
bind mounts, no network, a seccomp allow-list, a hard timeout for the whole process tree, and its
output captured up to a cap. It is for code from your own repositories (builds, tests, generated
programs). Code from anyone else belongs in a microVM, not here: a kernel bug in an allowed system call
is an escape.

Linux only (x86-64 and arm64). On macOS the package compiles and `Run` fails.

## API

```tin
import "sandbox"

fn build() !sandbox.Result {
	return try sandbox.Run(sandbox.Spec{
		Argv: []str{"/tin/bin/tinc", "-o", "/scratch/app", "/src/app.tin"},
		Env: []str{"TIN_ROOT=/tin", "PATH=/usr/bin:/bin"},
		Dir: "",
		Inputs: []sandbox.Bind{sandbox.Bind{From: "/srv/tin", To: "/tin"}, sandbox.Bind{From: "/srv/repo", To: "/src"}},
		System: []str{},
		NoSystem: false,
		Scratch: "/srv/work/42",
		ScratchPath: "",
		Memory: 256mb,
		CPU: 1.0,
		Pids: 64,
		Weight: 50,
		Timeout: 2 * 60 * 1000000000,
		MaxOutput: 1mb,
		Cgroup: "/sys/fs/cgroup/app.service/sandboxes",
		Network: false,
	})
}
```

- `Run(spec) !Result`: builds the sandbox, runs `Argv`, reads stdout and stderr while it runs and waits for
  it through the scheduler (other tasks keep running). The result:
  - `Code`: the exit code, or -1 when a signal ended the program; `Signal`: that signal, or 0.
  - `TimedOut`: the timeout killed the tree. `OOM`: memory.max killed a process (`memory.events`
    `oom_kill`; with `memory.oom.group` the whole sandbox goes).
  - `Stdout`, `Stderr`: at most `MaxOutput` bytes each; `StdoutTruncated`, `StderrTruncated` when more
    came (the rest is read and dropped, so the program never blocks on a full pipe).

  A program that fails, dies of a signal, times out or is killed for memory is a `Result`. A sandbox that
  cannot be built (no user namespaces, a missing input, a cgroup that is not delegated) or a program that
  cannot be executed is a fault naming the step, such as
  `sandbox: exec /no/such: no such file or directory`. A task deadline (`within`) kills the tree like the
  timeout and fails with `fault.DeadlineExceeded`.
- `Spec` fields, zero meaning the default:
  - `Argv`: the program and its arguments. A name without a slash is looked up in the `PATH` of `Env`
    (else `/usr/local/bin:/usr/bin:/bin`) inside the sandbox.
  - `Env`: exactly these `NAME=value` entries; none is an empty environment.
  - `Dir`: the working directory inside; the scratch path, or `/` without scratch.
  - `Inputs`: host files or directories (symlinks followed) mounted read-only at `To`.
  - `System`: host directories mounted read-only at the same path; empty is `DefaultSystem()`
    (`/usr`, `/lib`, `/lib64`, `/bin`, `/sbin`; a missing one is skipped, a symlink such as merged
    `/usr`'s `/bin` is recreated as the same symlink). `NoSystem` mounts none (a static program, such as
    any Tin program).
  - `Scratch`: a host directory mounted read-write at `ScratchPath` (`/scratch`).
  - `Memory` (memory.max, bytes; memory.swap.max is set to 0), `CPU` (cpu.max in cores over a 100 ms
    period), `Pids` (pids.max, processes and threads), `Weight` (cpu.weight, 1 to 10000): need `Cgroup`.
  - `Timeout`: nanoseconds before the whole tree is killed; `DefaultTimeout` is one minute.
  - `MaxOutput`: bytes kept of stdout and of stderr each; `DefaultMaxOutput` is 1 MiB.
  - `Cgroup`: the cgroup v2 directory each run makes its own child cgroup in (see below), or `""`.
  - `Network`: share the caller's network namespace and allow IPv4 and IPv6 sockets. The default is no
    network at all.

## What the program sees

- **Namespaces**: new user, mount, PID, network, IPC, UTS and cgroup namespaces. The program runs as uid
  and gid 1000, mapped to the caller's own (a one-line map, so an unprivileged caller can make it). Its
  hostname is `sandbox`. It is PID 2: PID 1 is the sandbox's init, which reaps orphans and leaves when the
  program exits, so the kernel kills whatever the program left behind.
- **Files**: a new root (pivot_root): a read-only tmpfs holding only the system directories, the inputs
  (read-only, nosuid, nodev, recursively), the scratch directory (read-write, nosuid, nodev), a fresh
  `/proc` for its PID namespace (with `/proc/sys`, `/proc/sysrq-trigger`, `/proc/irq` and `/proc/bus`
  read-only), and a `/dev` with `null`, `zero`, `full`, `random`, `urandom` and the `fd`, `stdin`,
  `stdout`, `stderr` links. Nothing else of the host is mounted: no `/etc`, `/home`, `/tmp`, `/sys`.
  stdin is `/dev/null`.
- **Network**: none: only a loopback interface, which is down.
- **Capabilities**: none. The uid inside is not 0, so execve drops the capability set the user namespace
  gave the setup, and `PR_SET_NO_NEW_PRIVS` is set.
- **seccomp**: an allow-list of about 260 system calls (x86-64) or 230 (arm64) that ordinary static and
  dynamic programs need (the Tin compiler, `/bin/sh`, coreutils, Go programs). A call outside it fails with
  EPERM: mount and the new mount API, `pivot_root`, `chroot`, `ptrace`, `process_vm_readv`/`writev`,
  `kexec_*`, `bpf`, `perf_event_open`, `userfaultfd`, `unshare`, `setns`, `keyctl`, `add_key`,
  `request_key`, the module calls, `reboot`, `swapon`, the clock setters, `open_by_handle_at`, io_uring
  (the Tin runtime falls back to helper threads), `personality` and the rest. `socket` and `socketpair`
  are allowed for `AF_UNIX` only (and `AF_INET`, `AF_INET6` with `Network`); `clone` without any
  `CLONE_NEW*` flag; `clone3` gets ENOSYS, so a C library falls back to `clone`, whose flags the filter
  can see. A system call from another ABI (i386 or x32 on x86-64, arm32 on arm64) kills the process. The
  table is kept by name in `sandbox_linux.tin`; the numbers are per architecture in
  `sandbox_linux_amd64.tin` and `sandbox_linux_arm64.tin`.
- **Limits**: with `Cgroup`, a child cgroup `tin-sandbox-PID-CORE-N` is made before the program starts
  (`clone3` with `CLONE_INTO_CGROUP`, so not one instruction runs outside it), the limits are written, and
  after the program exits the cgroup is killed (`cgroup.kill`) and removed. The timeout also writes
  `cgroup.kill`, and kills init.

## Requirements

- Linux 5.12 or later (clone3 with `CLONE_INTO_CGROUP`, `mount_setattr`, `cgroup.kill` from 5.14).
- User namespaces for the caller: root, or an unprivileged user where they are allowed
  (`user.max_user_namespaces` > 0, and on Ubuntu 23.10 and later
  `kernel.apparmor_restrict_unprivileged_userns=0` or an AppArmor profile that allows `userns`).
  Otherwise Run fails with `sandbox: clone3: operation not permitted (user namespaces need ...)`.
- For limits, a cgroup v2 directory delegated to the caller that holds no process of its own (the
  "no internal processes" rule), with the controllers enabled above it. Run enables `memory`, `cpu` and
  `pids` in the directory's `cgroup.subtree_control` when they are not yet. With systemd, a unit with
  `Delegate=yes` whose service process moves itself into a leaf (for example `app.service/main`) and
  passes `app.service/sandboxes`; when root, any directory under `/sys/fs/cgroup`.
- The sandboxed processes are the caller's own user, but without any capability on the host, so every
  input must be readable, and every directory above it enterable, by that user as a plain user. That holds
  for root too: running as root does not get the sandbox through a directory only its owner may enter
  (GitHub's `/home/runner` is 0750, which is why the check copies what it mounts to `/tmp`). The scratch
  directory must be writable by the caller's user.

## Tests

`sh tools/ci/tin.sh sandbox_check` (Linux; the CI step "Sandboxed processes ..." on both Linux runners)
builds `tools/ci/fixtures/sandbox_probe.tin` (a static Tin probe), `tools/ci/fixtures/sandbox_sys.go`
(a static Go probe of the denied system calls) and `tools/ci/fixtures/sandbox.tin`, copies what the
sandbox mounts to a fresh directory under `/tmp`, and runs the fixture as root (`sudo -n` when the check
is not root; root may make user namespaces and cgroups on GitHub's Ubuntu 24.04 runners, where
unprivileged user namespaces are restricted). As root the fixture makes `/sys/fs/cgroup/tin-sandbox-check`
with `memory`, `cpu` and `pids` enabled and uses it as `Cgroup`. It checks exit codes and signals, PATH
lookup, the output caps (10 MB written, 1 MiB kept), that the inputs are readable and read-only and the
scratch directory writable, that the repository, `/etc/shadow`, `/etc`, `/home`, `/root`, `/tmp`,
`/proc/1/environ` and `/proc/sys` cannot be read or written, that only a down loopback exists and neither
1.1.1.1:53 nor a TCP server on the host's 127.0.0.1 can be reached (and that `Network` reaches the
latter), that `socket(AF_INET)`, `AF_INET6`, `AF_NETLINK`, `unshare`, clone with `CLONE_NEWUSER`,
`mount`, `keyctl`, `io_uring_setup` and `ptrace` of a child fail with EPERM while a Go program runs, that
the timeout and a `within` deadline leave no process of the tree behind (found by a marker in their
argv), that the Tin compiler builds a program inside which then runs with no system directory, and with
cgroup v2: a run in a cgroup and the cgroup removed, the limits written, a loop and a fork bomb stopped
by `pids.max` (the check's own process still forks), and a 512 MiB allocation under a 64 MiB
`memory.max` reported as OOM. Without a cgroup v2 hierarchy at `/sys/fs/cgroup` (a cgroup v1 or hybrid
machine) the cgroup part is skipped, which the check refuses on CI.

## Known gaps

- `/proc/1/cmdline` inside shows the caller's command line: init is a copy of the calling process. Its
  environment and memory are not readable (init is not dumpable and holds capabilities the program lacks).
- With `Network`, the abstract Unix socket namespace is the host's too.
- No stdin input, no `/dev/shm`, no `/tmp` but the scratch directory (point `TMPDIR` at it).
- Only cgroup v2.
