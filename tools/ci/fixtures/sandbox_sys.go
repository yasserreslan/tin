//go:build linux

// The system-call probe of tools/ci/sandbox_check.tin (#1002): run inside packages/sandbox, it tries calls the seccomp filter denies
// and prints one line per call, "NAME ok" or "NAME <errno text>", so the check sees EPERM where the kernel alone would have allowed the
// call (socket(AF_INET), unshare(CLONE_NEWUSER), clone with CLONE_NEWUSER, keyctl, io_uring_setup, ptrace of its own child) and that
// the calls a Go program needs still work (it is a Go program: threads, signals, its runtime). Built static (CGO_ENABLED=0).
package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

const sysIoUringSetup = 425 // the same number on amd64 and arm64

func report(name string, err error) {
	if err == nil {
		fmt.Println(name, "ok")
		return
	}
	fmt.Println(name, err.Error())
}

func errnoOf(r1, r2 uintptr, e syscall.Errno) error {
	if e != 0 {
		return e
	}
	return nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "sleep" {
		select {}
	}
	// Outside a sandbox these calls would succeed (and mount over /tmp): the check sets this only for the sandboxed run.
	if os.Getenv("TIN_SANDBOX_PROBE") != "1" {
		fmt.Fprintln(os.Stderr, "sandbox_sys: run only inside the sandbox (TIN_SANDBOX_PROBE=1)")
		os.Exit(2)
	}
	report("getpid", errnoOf(syscall.RawSyscall(syscall.SYS_GETPID, 0, 0, 0)))
	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	report("socket-unix", err)
	if err == nil {
		syscall.Close(fd)
	}
	fd, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	report("socket-inet", err)
	if err == nil {
		syscall.Close(fd)
	}
	fd, err = syscall.Socket(syscall.AF_INET6, syscall.SOCK_DGRAM, 0)
	report("socket-inet6", err)
	if err == nil {
		syscall.Close(fd)
	}
	fd, err = syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, 0)
	report("socket-netlink", err)
	if err == nil {
		syscall.Close(fd)
	}
	report("unshare-user", syscall.Unshare(syscall.CLONE_NEWUSER))
	report("unshare-mount", syscall.Unshare(syscall.CLONE_NEWNS))
	// A fork into a new user namespace: if the filter let it through, the child exits at once.
	r1, _, e := syscall.RawSyscall6(syscall.SYS_CLONE, syscall.CLONE_NEWUSER|uintptr(syscall.SIGCHLD), 0, 0, 0, 0, 0)
	if e == 0 && r1 == 0 {
		syscall.RawSyscall(syscall.SYS_EXIT_GROUP, 0, 0, 0)
	}
	if e == 0 {
		var ws syscall.WaitStatus
		syscall.Wait4(int(r1), &ws, 0, nil)
	}
	report("clone-newuser", errnoOf(r1, 0, e))
	report("mount", syscall.Mount("none", "/tmp", "tmpfs", 0, ""))
	report("keyctl", errnoOf(syscall.Syscall(syscall.SYS_KEYCTL, 0, ^uintptr(2), 0))) // KEYCTL_GET_KEYRING_ID, KEY_SPEC_SESSION_KEYRING
	params := make([]byte, 120)
	r1, _, e = syscall.Syscall(sysIoUringSetup, 4, uintptr(unsafe.Pointer(&params[0])), 0)
	if e == 0 {
		syscall.Close(int(r1))
	}
	report("io_uring_setup", errnoOf(r1, 0, e))
	child := exec.Command(os.Args[0], "sleep")
	if err := child.Start(); err != nil {
		report("ptrace", err)
	} else {
		report("ptrace", syscall.PtraceAttach(child.Process.Pid))
		child.Process.Kill()
		child.Wait()
	}
}
