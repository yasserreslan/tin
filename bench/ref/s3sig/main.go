// Command s3sig signs a corpus of requests with the AWS SDK for Go v2's Signature Version 4 signer, the reference for
// packages/s3 (tools/ci/s3_check.tin feeds the same corpus to tools/ci/fixtures/s3_sig.tin and compares the lines).
//
// Each input line is one request, its fields hex-encoded ("-" for an empty one) and separated by spaces:
//
//	name method scheme host path nquery [name value]... nheaders [name value]... payloadhash contentlength
//	accesskey secretkey token region service unixtime expires
//
// The path is the object path unescaped: it is escaped as the S3 client escapes a key (smithy's httpbinding.EscapePath)
// and signed as it is (DisableURIPathEscaping, as S3 signs). expires 0 signs the Authorization header; above 0 it
// presigns the URL for that many seconds. The output is "name Authorization" or "name URL", one line per request.
package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/smithy-go/encoding/httpbinding"
)

func field(s string) string {
	if s == "-" {
		return ""
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<24)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	signer := v4.NewSigner(func(o *v4.SignerOptions) { o.DisableURIPathEscaping = true })
	for in.Scan() {
		f := strings.Fields(in.Text())
		if len(f) == 0 {
			continue
		}
		at := 0
		next := func() string { at++; return field(f[at-1]) }
		num := func() int64 {
			n, err := strconv.ParseInt(next(), 10, 64)
			if err != nil {
				panic(err)
			}
			return n
		}
		name, method, scheme, host, path := next(), next(), next(), next(), next()
		query := url.Values{}
		for i, n := int64(0), num(); i < n; i++ {
			k := next()
			query.Add(k, next())
		}
		header := http.Header{}
		for i, n := int64(0), num(); i < n; i++ {
			k := next()
			header.Add(k, next())
		}
		payload, length := next(), num()
		creds := aws.Credentials{AccessKeyID: next(), SecretAccessKey: next(), SessionToken: next()}
		region, service, unix, expires := next(), next(), num(), num()
		u := &url.URL{Scheme: scheme, Host: host, Path: path, RawPath: httpbinding.EscapePath(path, false)}
		if expires > 0 {
			query.Set("X-Amz-Expires", strconv.FormatInt(expires, 10))
		}
		u.RawQuery = query.Encode()
		req := &http.Request{Method: method, URL: u, Header: header, ContentLength: length}
		when := time.Unix(unix, 0)
		if expires > 0 {
			signed, _, err := signer.PresignHTTP(context.Background(), creds, req, payload, service, region, when)
			if err != nil {
				panic(err)
			}
			fmt.Fprintln(out, name, signed)
			continue
		}
		if err := signer.SignHTTP(context.Background(), creds, req, payload, service, region, when); err != nil {
			panic(err)
		}
		fmt.Fprintln(out, name, req.Header.Get("Authorization"))
	}
	if err := in.Err(); err != nil {
		panic(err)
	}
}
