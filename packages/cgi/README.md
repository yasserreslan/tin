# cgi and cgi/fcgi

CGI (RFC 3875) and FastCGI handler adapters over an anvil handler, following Go's `net/http/cgi` and
`net/http/fcgi`. The handler is an anvil `Router`, or a plain `fn(Req, mut Out)` through `ServeFunc`.

## API

`cgi`:

- `Serve(r anvil.Router) !`: runs r as a CGI program: the request from the environment and standard input, the response to standard output.
- `ServeFunc(h fn(anvil.Req, mut anvil.Out)) !`: Serve for one handler on every path and method.
- `Call(r anvil.Router, env map[str]str, body str) !anvil.Out`: the request the CGI variables describe, through r (Go's `RequestFromMap`, then the handler).
- `Format(code i64, headers []str, body str, fastcgi bool) str`: the response bytes: the `Status` line, the fields sorted by canonical name, a blank line, the body.

`cgi/fcgi`:

- `Serve(l wire.Listener, r anvil.Router) !`: answers FastCGI requests on l with r.
- `ServeFunc(l wire.Listener, h fn(anvil.Req, mut anvil.Out)) !`: Serve for one handler.

## Design notes

- `Call` builds the request from the variables as Go's `cgi.RequestFromMap` does, and runs it through
  `Router.RunWith`, anvil's in-process path, so routing, middleware and 404/405 are anvil's.
- `CONTENT_LENGTH` is a signed integer (Go's `ParseInt`); a negative or empty value means no body, a
  value that is not an integer fails with Go's message. `Serve` reads at most that many bytes of stdin.
- `REQUEST_URI` is the target; without it, `SCRIPT_NAME`, `PATH_INFO` and `QUERY_STRING` are joined.
- `HTTP_*` variables become header fields named as Go spells them (underscores to hyphens, canonical case).
  `HTTP_HOST` is the `Host` field, which `q.Header("Host")` returns.
- The environment is read from `/proc/self/environ` on Linux and from `_NSGetEnviron` on macOS: quarry has no
  listing of all variables.
- `Format` writes what Go's child writes: `Status: <code> <reason>` (the reason from Go's table, empty when
  Go has none), the fields sorted by canonical name with the values of one name in the order set, and a blank
  line. The FastCGI form adds a `Date` (unless the handler set one) and drops Content-Type on 304, as Go's fcgi does.
- A write to a closed standard output ends the CGI program with SIGPIPE, as Go does (anvil's runtime ignores
  SIGPIPE, so `Serve` re-raises it with the default disposition).
- FastCGI: the records of the protocol as Go's `net/http/fcgi` handles them: BEGIN, PARAMS, STDIN, STDOUT,
  STDERR, END_REQUEST, GET_VALUES, UNKNOWN_TYPE, ABORT and DATA, the 8-byte header with its padding, and
  the 1- or 4-byte name and value lengths. A malformed record closes the connection, as Go's child does.

## Verified against Go

`tools/ci/cgi_check.tin` (`sh tools/ci/tin.sh cgi_check`) runs `tools/ci/fixtures/cgi_twin.tin` as a CGI
program and as a FastCGI responder, and `bench/ref/cgi` (Go's `net/http/cgi` and `net/http/fcgi`, the same
routes) on 82 CGI requests and 20 FastCGI connections: the bytes of each response, the exit status and the
stderr of malformed requests must be equal (the FastCGI `Date` is normalized). The strict test is
`toolchain/tests/v2/cgi.tin`. The FastCGI transcripts were also compared on Linux arm64 (the Tin server in a
container, the probe in the same container).

## Known gaps

- No content sniffing: a response without a Content-Type gets `text/plain; charset=utf-8`, where Go's
  `DetectContentType` may give another type for HTML or binary bodies.
- anvil's `Head` drops Content-Length, Transfer-Encoding and Connection, which anvil owns; Go's child passes them through.
- `RemoteAddr`, `ClientIP` and `TLSConn` are empty: an in-process request has no connection. REMOTE_ADDR, REMOTE_PORT and HTTPS are not visible to a handler.
- `Req.Path` is the raw path; Go's `URL.Path` is decoded. The query is raw in both.
- Go's URL parse errors (`cgi: failed to parse REQUEST_URI into a URL`) are not reproduced.
- `Req.Proto()` is always `HTTP/1.1` in-process; Go's `Proto` is SERVER_PROTOCOL.
- A body in `Call` is used as given; `Serve` bounds it to CONTENT_LENGTH. Under FastCGI the whole STDIN is the body, as in Go.
- FastCGI serves one connection at a time and runs a connection's requests in order. A task spawned per connection
  does not run while the accept loop waits in wire (see `packages/rpc`), so Go's concurrent serving is not reproduced.
- FastCGI ABORT_REQUEST after a request's first STDIN record: Go would still run the handler; here the handler runs
  only when stdin ends, so an aborted request is dropped.
- Only the responder role. Filter and authorizer requests get END_REQUEST with UNKNOWN_ROLE.
