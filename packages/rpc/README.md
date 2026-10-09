# rpc: Go's net/rpc, and net/rpc/jsonrpc

`import "rpc"` is Go's `net/rpc` over a codec: the `Request` and `Response` headers (`ServiceMethod`, `Seq`, `Error`), a
`Server` that serves registered methods, and a `Client` with `Call` and `Go`. `import "rpc/jsonrpc"` is Go's
`net/rpc/jsonrpc`: the JSON-RPC wire over a stream (a `wire` connection or an `io.Pipe` pair), with `argo` for the JSON.

The wire is Go's. A request is `{"method":"Service.Method","params":[args],"id":seq}`, a response is
`{"id":...,"result":...,"error":...}` with a `null` for the member not used, and each message is followed by a newline,
as Go's encoder writes it. The twin check (`tools/ci/rpc_check.tin`) runs Tin's client against Go's server and Go's client
against Tin's, over TCP.

## API

Package `rpc`:

- `Request`, `Response`: the headers (`ServiceMethod str`, `Seq u64`, and `Error str` in a response).
- `ServerCodec`, `ClientCodec`: the codec shapes. A server codec reads a request header and its parameters, and writes a
  response; a client codec writes a request and reads a response header and its result. Bodies are JSON text.
- `NewServer() Server`, `RegisterMethod[A, R](srv mut Server, name str, h fn(A) !R) !`, `(srv Server) ServeCodec(codec dyn ServerCodec)`.
- `NewClient(codec dyn ClientCodec) Client`, `(c mut Client) Input()` (the reader), `(c mut Client) Close() !`.
- `Go[A, R](c mut Client, method str, args A) Pending[R]`, `Call[A, R](c mut Client, method str, args A) !R`,
  `(p mut Pending[R]) Wait() !R`.
- `ErrShutdown` (a call on a closed client), `EOF` and `ErrUnexpectedEOF` (what a codec reports when its stream ends).

Package `rpc/jsonrpc`:

- `Stream`: the transport shape (`Read`, `Write`, `CloseWrite`, `Close`).
- `WireStream(c wire.Conn) dyn Stream`, `PipeStream(r io.PipeReader, w io.PipeWriter) dyn Stream`: the two transports.
- `NewServerCodec(conn dyn Stream) dyn rpc.ServerCodec`, `NewClientCodec(conn dyn Stream) dyn rpc.ClientCodec`,
  `NewClient(conn dyn Stream) rpc.Client`.
- `ServeConn(srv rpc.Server, conn dyn Stream)`, `Dial(addr str) !rpc.Client`, `Serve(srv rpc.Server, l wire.Listener)`.

## Registration

Tin has no reflection, so Go's `Register(rcvr)` is not ported. A method is registered by its name and a typed handler, and
the argument and reply types come from the handler:

    mut srv = rpc.NewServer()
    try rpc.RegisterMethod(mut srv, "Arith.Multiply", fn(a Args) !Reply {
        return Reply{C: a.A * a.B}
    })

A handler returns its reply, or fails with `fail "text"`; the text becomes the response's `Error`, as Go's error string does.
The registry is a map of `dyn` method values, so the server does not need the argument types at compile time.

A call's parameters are the JSON array of its arguments. The handler gets the first element, as Go's jsonrpc decodes
`params[0]`; an empty array leaves the argument at its zero value. The client sends the argument as a one-element array.

## Design

- **Codecs move JSON text.** A server codec returns the parameters as text, and a client codec returns the result as text;
  the typed layer decodes and encodes them with `argo`. A codec never needs the argument types.
- **Go's texts.** An unknown service, an unknown method and a name with no dot fail with Go's texts:
  `rpc: can't find service X`, `rpc: can't find method X`, `rpc: service/method request ill-formed: X`. A request
  without params fails with `jsonrpc: request body missing params`. A request header that cannot be read ends the
  connection, as in Go.
- **Messages.** The jsonrpc codec finds the end of each message by its braces, so one read may hold several messages or a
  message may come in pieces. A request's `id` is kept as its raw JSON and echoed as it arrived. Strings are escaped as
  Go's encoder escapes them (`<`, `>`, `&` and U+2028/U+2029 included).
- **Calls and replies.** Each call has a waiter: a lane of one, and the reply or the failure. The client's reader task
  matches responses to waiters by sequence number. A lane of a struct does not compile (its `RecvOk` needs a zero value
  the struct cannot give), so the reply and failure sit in the waiter, and the lane carries only the wake-up.
- **Tasks.** The client's reader is a task of the caller's scope, started with `s.spawn(fn() ! { c.Input() })`: a handle
  cannot outlive its scope, so the caller owns the reader. `Close` half-closes the request side, so the reader ends when
  the peer closes its side, as Go's client does when its connection ends.
- **Concurrency.** The server reads a connection's requests in order and answers each before it reads the next one. The
  listener serves one connection at a time. The reason is what the runs made while building this showed: a task spawned
  in the accept loop or in the call loop did not run while its parent waited in a wire read, and `s.yield()` in the call
  loop did not return. The client needs no such change, because its calls park on lanes, and its reader runs then.

## Known gaps

- **The gob codec.** Go's `net/rpc` uses gob by default, and this package has no gob codec: it needs #913. `ServeCodec`
  takes any codec, so the gob codec plugs in without change here.
- **One call at a time per connection, and per listener.** Go serves each call in its own goroutine, and each connection
  in its own goroutine. Here a connection's calls run in order and the listener serves connections one at a time. A
  concurrent server needs the scheduler to run spawned tasks while its parent waits in a wire read; that is runtime work.
- **Parameters.** The handler's argument is decoded from the whole params array, so the elements after the first are decoded
  too (Go ignores them). A bad element after the first fails the call where Go would not.
- **Error texts of malformed JSON.** A message that is not valid JSON fails with a Tin fault, not with Go's
  `json:` error text. A response with a non-string `error` gives `invalid error <raw text>`; Go formats the value with `%v`.
- **Reply decoding.** A result that does not decode into the reply type fails the call in `Wait`, where Go would shut the
  connection down. Other calls on the connection keep working here.
- **Encoding.** A string with invalid UTF-8 is written as is; Go writes U+FFFD. Whitespace inside a request's `id` is kept;
  Go compacts it.
- **`Register(rcvr)`.** Not ported (no reflection). Methods are registered one by one, with their argument and reply types.

## Tests

- `toolchain/tests/v2/rpc.tin` (with `.out`): a server and a client in one program over two `io.Pipe`s, every outcome
  printed (calls, handler errors, the unknown-name texts, pipelined calls, calls after `Close`), and four raw requests whose
  response bytes are printed. Verified against `bench/ref/rpc/main.go` (Go's `net/rpc` on the same calls): identical output.
- `tools/ci/rpc_check.tin` (`sh tools/ci/tin.sh rpc_check`): Tin's server and client over TCP, and Go's server and client
  over TCP in both pairings, each printing the 19 session lines of `tools/ci/fixtures/rpc_twin.out`. The Go pairings run
  when `go` is installed.
