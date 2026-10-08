package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

const modified = int64(1700000000)

func write(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := zip.NewWriter(f)
	w.SetComment("zip twin")
	for _, entry := range []struct {
		name, body string
		method     uint16
		mode       os.FileMode
	}{
		{"hello.txt", "hello, zip", zip.Deflate, 0644},
		{"raw.txt", "stored", zip.Store, 0640},
		{"café.txt", "unicode", zip.Deflate, 0600},
		{"link", "target", zip.Store, os.ModeSymlink | 0777},
		{"folder/", "", zip.Store, os.ModeDir | 0755},
	} {
		h := &zip.FileHeader{Name: entry.name, Method: entry.method}
		h.Comment = "file comment"
		h.SetModTime(time.Unix(modified, 0).UTC())
		h.SetMode(entry.mode)
		out, err := w.CreateHeader(h)
		if err != nil {
			return err
		}
		if _, err = io.WriteString(out, entry.body); err != nil {
			return err
		}
	}
	if err := w.Close(); err != nil {
		return err
	}
	return f.Close()
}

func read(path string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()
	fmt.Printf("%d %s\n", len(r.File), r.Comment)
	for _, f := range r.File {
		body, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(body)
		closeErr := body.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		_, offset := f.Modified.Zone()
		fmt.Printf("%s | %s | %t | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %x | %d\n%s\n",
			f.Name, f.Comment, f.NonUTF8, f.CreatorVersion, f.ReaderVersion, f.Flags, f.Method,
			f.Modified.UnixNano(), offset, f.ModifiedTime, f.ModifiedDate, f.CRC32,
			f.CompressedSize, f.UncompressedSize, f.CompressedSize64, f.UncompressedSize64,
			f.ExternalAttrs, f.Extra, uint32(f.Mode()), data)
	}
	return nil
}

func faultKind(err error) string {
	switch {
	case errors.Is(err, zip.ErrChecksum):
		return "checksum"
	case errors.Is(err, zip.ErrFormat):
		return "format"
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "eof"
	default:
		return "other"
	}
}

func classify(path string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		fmt.Println(faultKind(err))
		return nil
	}
	defer r.Close()
	for _, f := range r.File {
		body, err := f.Open()
		if err != nil {
			fmt.Println(faultKind(err))
			return nil
		}
		_, err = io.Copy(io.Discard, body)
		closeErr := body.Close()
		if err != nil {
			fmt.Println(faultKind(err))
			return nil
		}
		if closeErr != nil {
			fmt.Println(faultKind(closeErr))
			return nil
		}
	}
	fmt.Println("ok")
	return nil
}

func main() {
	if len(os.Args) != 3 {
		panic("usage: zip-ref write|read|classify path")
	}
	var err error
	switch os.Args[1] {
	case "write":
		err = write(os.Args[2])
	case "read":
		err = read(os.Args[2])
	case "classify":
		err = classify(os.Args[2])
	default:
		panic("unknown operation")
	}
	if err != nil {
		panic(err)
	}
}
