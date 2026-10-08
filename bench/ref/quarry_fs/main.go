package main

// Go twin of toolchain/tests/v2/quarry_fs.tin: the same files, the same lines.

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

func typ(m fs.FileMode) string {
	switch {
	case m.IsDir():
		return "Dir"
	case m&fs.ModeSymlink != 0:
		return "Symlink"
	case m&fs.ModeNamedPipe != 0:
		return "Fifo"
	case m&fs.ModeSocket != 0:
		return "Socket"
	case m&fs.ModeDevice != 0:
		return "Device"
	}
	return "Regular"
}

func show(label, path string, follow bool) {
	var fi fs.FileInfo
	var err error
	if follow {
		fi, err = os.Stat(path)
	} else {
		fi, err = os.Lstat(path)
	}
	if err != nil {
		fmt.Println(label, "error:", err)
		return
	}
	st := fi.Sys().(*syscall.Stat_t)
	times := fi.ModTime().UnixNano() > 0 && st.Ino > 0
	t := typ(fi.Mode())
	perm := fmt.Sprintf("%o", uint32(fi.Mode().Perm())|uint32(st.Mode)&0o7000)
	switch t {
	case "Dir":
		fmt.Println(label, t, perm, times)
	case "Symlink":
		fmt.Println(label, t, fi.Size(), st.Nlink, times)
	default:
		fmt.Println(label, t, perm, fi.Size(), st.Nlink, times)
	}
}

func main() {
	d := "/tmp/tin-test-quarry-fs"
	os.RemoveAll(d)
	os.MkdirAll(d+"/sub/deeper", 0o755)
	os.WriteFile(d+"/a.txt", []byte("hello, tin"), 0o644)
	os.WriteFile(d+"/sub/b", nil, 0o644)
	os.Symlink("a.txt", d+"/link")
	os.Symlink("nowhere", d+"/dangling")
	os.Chmod(d+"/a.txt", 0o755)
	os.Chmod(d+"/sub/b", 0)
	show("file", d+"/a.txt", false)
	show("no-perm file", d+"/sub/b", false)
	show("dir", d+"/sub", false)
	show("link lstat", d+"/link", false)
	show("link stat", d+"/link", true)
	show("dangling lstat", d+"/dangling", false)
	show("dangling stat", d+"/dangling", true)
	show("missing", d+"/missing", false)
	a, _ := os.Lstat(d + "/a.txt")
	v, _ := os.Stat(d + "/link")
	sa, sv := a.Sys().(*syscall.Stat_t), v.Sys().(*syscall.Stat_t)
	fmt.Println("same inode through the link:", sa.Ino == sv.Ino && sa.Dev == sv.Dev)
	t1, _ := os.Readlink(d + "/link")
	fmt.Println("readlink:", t1)
	_, e1 := os.Readlink(d + "/a.txt")
	fmt.Println("readlink of a file:", e1)
	e2 := os.Symlink("x", d+"/link")
	fmt.Println("symlink over a name:", e2 != nil)
	es, _ := os.ReadDir(d)
	for _, e := range es {
		fmt.Println("entry", e.Name(), typ(e.Type()))
	}
	_, e3 := os.ReadDir(d + "/a.txt")
	fmt.Println("entries of a file:", e3 != nil)
	_, e4 := os.Open(d + "/missing")
	fmt.Println("sync missing:", e4)
	f, _ := os.OpenFile(d+"/lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	f.WriteString("4242\n")
	f.Sync()
	f.Close()
	_, e5 := os.OpenFile(d+"/lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	fmt.Println("second exclusive create fails:", e5 != nil)
	lock, _ := os.ReadFile(d + "/lock")
	fmt.Println("lock holds:", len(lock))
	os.Chmod(d+"/sub/b", 0o644)
	os.RemoveAll(d)
}
