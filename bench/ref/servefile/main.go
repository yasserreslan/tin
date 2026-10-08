package main

// The ServeFile twin, Go side: serve one file at /f with http.ServeContent on the port given as the second argument, with the same
// ETag and Content-Type as anvil's ServeFile, so that tools/ci/servefile_check.tin can send both servers the same requests and
// compare the answers (#823). Usage: servefile FILE PORT.

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	path, port := os.Args[1], os.Args[2]
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	http.HandleFunc("/f", func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open(path)
		if err != nil {
			http.Error(w, "nope", 404)
			return
		}
		defer f.Close()
		w.Header().Set("ETag", fmt.Sprintf("\"%x-%x\"", info.ModTime().Unix(), info.Size()))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeContent(w, r, path, info.ModTime(), f)
	})
	if err := http.ListenAndServe("127.0.0.1:"+port, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
