package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"os"
	"strconv"
	"strings"
)

func traced(url string, client *http.Client) (string, string, error) {
	var events []string
	trace := &httptrace.ClientTrace{
		GetConn:           func(string) { events = append(events, "get") },
		TLSHandshakeStart: func() { events = append(events, "tls-start") },
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			if err != nil {
				events = append(events, "tls-failed")
			} else {
				events = append(events, "tls-done")
			}
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err != nil {
				events = append(events, "write-failed")
			} else {
				events = append(events, "wrote")
			}
		},
		GotFirstResponseByte: func() { events = append(events, "first") },
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", "", err
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	return strings.Join(events, ",") + ",", strconv.Itoa(resp.StatusCode) + " " + string(body), nil
}

func main() {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "trace-ok") })
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		fs := flag.NewFlagSet("serve", flag.ExitOnError)
		plain := fs.String("plain", "", "plain HTTP address")
		secure := fs.String("tls", "", "TLS address")
		cert := fs.String("cert", "", "certificate file")
		key := fs.String("key", "", "key file")
		_ = fs.Parse(os.Args[2:])
		go func() {
			if err := http.ListenAndServe(*plain, h); err != nil {
				panic(err)
			}
		}()
		if err := http.ListenAndServeTLS(*secure, *cert, *key, h); err != nil {
			panic(err)
		}
		return
	}
	plain := httptest.NewServer(h)
	defer plain.Close()
	plainEvents, plainResponse, err := traced(plain.URL, plain.Client())
	if err != nil {
		panic(err)
	}
	tlsServer := httptest.NewTLSServer(h)
	defer tlsServer.Close()
	tlsEvents, tlsResponse, err := traced(tlsServer.URL, tlsServer.Client())
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s %s\n%s %s\n", plainEvents, plainResponse, tlsEvents, tlsResponse)
}
