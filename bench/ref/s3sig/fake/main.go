// Command fake is an in-memory S3 server for tools/ci/s3_check.tin: github.com/johannesboyne/gofakes3 behind a check of
// every request's Signature Version 4, recomputed with the AWS SDK for Go v2's signer from what came on the wire (the
// Authorization header or a presigned URL's query), and of every signed payload hash against the body.
//
//	fake ACCESSKEY SECRETKEY [TOKEN]
//
// It listens on 127.0.0.1 on a free port and prints the port on its first line. Requests for a key holding "flaky503"
// are answered 503 SlowDown twice per method and key before they reach the store, so the client's retries are seen; the
// first completion of a multipart upload whose key holds "complete200" is answered 200 with an InternalError body, as
// S3 may.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

var (
	creds  aws.Credentials
	signer = v4.NewSigner(func(o *v4.SignerOptions) { o.DisableURIPathEscaping = true })
	mu     sync.Mutex
	flaky  = map[string]int{}
)

func refuse(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	fmt.Fprintf(w, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<Error><Code>%s</Code><Message>%s</Message><RequestId>fake</RequestId></Error>", code, msg)
}

// rebuilt is the request as the signer sees it: method, the path as it came, the query, and only the given headers.
func rebuilt(r *http.Request, query url.Values, names []string) *http.Request {
	u := &url.URL{Scheme: "http", Host: r.Host, Path: r.URL.Path, RawPath: r.URL.EscapedPath(), RawQuery: query.Encode()}
	req := &http.Request{Method: r.Method, URL: u, Header: http.Header{}, Host: r.Host}
	for _, n := range names {
		switch n {
		case "host", "x-amz-date", "x-amz-security-token":
		case "content-length":
			req.ContentLength = r.ContentLength
		default:
			for _, v := range r.Header.Values(n) {
				req.Header.Add(n, v)
			}
		}
	}
	return req
}

// checkHeader verifies an Authorization header; "" when it is right.
func checkHeader(r *http.Request, auth string) string {
	const pre = "AWS4-HMAC-SHA256 "
	if !strings.HasPrefix(auth, pre) {
		return "not AWS4-HMAC-SHA256"
	}
	parts := map[string]string{}
	for _, p := range strings.Split(auth[len(pre):], ", ") {
		k, v, _ := strings.Cut(p, "=")
		parts[k] = v
	}
	scope := strings.Split(parts["Credential"], "/")
	if len(scope) != 5 || scope[0] != creds.AccessKeyID {
		return "bad credential " + parts["Credential"]
	}
	if r.Header.Get("X-Amz-Security-Token") != creds.SessionToken {
		return "wrong session token"
	}
	when, err := time.Parse("20060102T150405Z", r.Header.Get("X-Amz-Date"))
	if err != nil {
		return "bad X-Amz-Date"
	}
	payload := r.Header.Get("X-Amz-Content-Sha256")
	if payload == "" {
		return "no X-Amz-Content-Sha256"
	}
	names := strings.Split(parts["SignedHeaders"], ";")
	req := rebuilt(r, r.URL.Query(), names)
	if err := signer.SignHTTP(context.Background(), creds, req, payload, scope[3], scope[2], when); err != nil {
		return err.Error()
	}
	if got := req.Header.Get("Authorization"); got != auth {
		return "signature mismatch: want " + got
	}
	return ""
}

// checkQuery verifies a presigned URL; "" when it is right.
func checkQuery(r *http.Request) string {
	q := r.URL.Query()
	sig := q.Get("X-Amz-Signature")
	scope := strings.Split(q.Get("X-Amz-Credential"), "/")
	if len(scope) != 5 || scope[0] != creds.AccessKeyID {
		return "bad credential"
	}
	when, err := time.Parse("20060102T150405Z", q.Get("X-Amz-Date"))
	if err != nil {
		return "bad X-Amz-Date"
	}
	expires, _ := strconv.Atoi(q.Get("X-Amz-Expires"))
	if time.Now().After(when.Add(time.Duration(expires) * time.Second)) {
		return "expired"
	}
	names := strings.Split(q.Get("X-Amz-SignedHeaders"), ";")
	rest := url.Values{}
	for k, vs := range q {
		switch k {
		case "X-Amz-Signature", "X-Amz-Algorithm", "X-Amz-Credential", "X-Amz-Date", "X-Amz-SignedHeaders", "X-Amz-Security-Token":
		default:
			rest[k] = vs
		}
	}
	req := rebuilt(r, rest, names)
	signed, _, err := signer.PresignHTTP(context.Background(), creds, req, "UNSIGNED-PAYLOAD", scope[3], scope[2], when)
	if err != nil {
		return err.Error()
	}
	u, _ := url.Parse(signed)
	if u.Query().Get("X-Amz-Signature") != sig {
		return "presigned signature mismatch"
	}
	return ""
}

// hashed checks a body against its signed SHA-256 as it is read, and cuts the connection when they differ.
type hashed struct {
	r    io.ReadCloser
	h    hash.Hash
	want string
}

func (b *hashed) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.h.Write(p[:n])
	if err == io.EOF && hex.EncodeToString(b.h.Sum(nil)) != b.want {
		panic(http.ErrAbortHandler)
	}
	return n, err
}

func (b *hashed) Close() error { return b.r.Close() }

func verify(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var why string
		if r.URL.Query().Get("X-Amz-Signature") != "" {
			why = checkQuery(r)
		} else {
			why = checkHeader(r, r.Header.Get("Authorization"))
		}
		if why != "" {
			fmt.Fprintln(os.Stderr, "fake: refused", r.Method, r.URL.String(), why)
			refuse(w, 403, "SignatureDoesNotMatch", why)
			return
		}
		if strings.Contains(r.URL.Path, "flaky503") {
			mu.Lock()
			k := r.Method + " " + r.URL.Path
			flaky[k]++
			n := flaky[k]
			mu.Unlock()
			if n <= 2 {
				refuse(w, 503, "SlowDown", "Please reduce your request rate.")
				return
			}
		}
		if strings.Contains(r.URL.Path, "complete200") && r.Method == "POST" && r.URL.Query().Has("uploadId") {
			mu.Lock()
			k := "complete " + r.URL.Path
			flaky[k]++
			n := flaky[k]
			mu.Unlock()
			if n == 1 {
				w.Header().Set("Content-Type", "application/xml")
				fmt.Fprint(w, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<Error><Code>InternalError</Code><Message>We encountered an internal error. Please try again.</Message></Error>")
				return
			}
		}
		if p := r.Header.Get("X-Amz-Content-Sha256"); p != "" && p != "UNSIGNED-PAYLOAD" && r.Body != nil {
			r.Body = &hashed{r: r.Body, h: sha256.New(), want: p}
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: fake ACCESSKEY SECRETKEY [TOKEN]")
		os.Exit(2)
	}
	creds = aws.Credentials{AccessKeyID: os.Args[1], SecretAccessKey: os.Args[2]}
	if len(os.Args) > 3 {
		creds.SessionToken = os.Args[3]
	}
	faker := gofakes3.New(s3mem.New())
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	fmt.Println(ln.Addr().(*net.TCPAddr).Port)
	if err := http.Serve(ln, verify(faker.Server())); err != nil {
		panic(err)
	}
}
