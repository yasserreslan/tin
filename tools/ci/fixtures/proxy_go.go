package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

func main() {
	target, err := url.Parse(os.Getenv("UPSTREAM"))
	if err != nil {
		panic(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	if err := http.ListenAndServe(":"+os.Getenv("PORT"), proxy); err != nil {
		panic(err)
	}
}
