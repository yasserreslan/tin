# rpc: Go's net/rpc

`import "rpc"` is Go's `net/rpc` over a codec: the `Request` and `Response` headers (`ServiceMethod`,
`Seq`, `Error`), a `Server` that serves methods and a `Client` with `Call` and `Go`. The wire is
whatever the codec writes; `rpc/jsonrpc` is Go's `net/rpc/jsonrpc` wire over a stream.

Status: in progress (part of #922). The API below is the plan; the README is updated with the code.

## Registration: explicit and typed

Tin has no runtime reflection, so Go's `Register(rcvr)` cannot be ported. A method is registered
by name with a typed handler, and the argument and reply types come from the registration:

    srv := rpc.NewServer()
    rpc.RegisterMethod(mut srv, "Arith.Multiply", fn(a Args) !Reply {
        return Reply{C: a.A * a.B}
    })

A handler returns its reply (`return v`) or fails (`fail "msg"`); a failure's text becomes the
response's `Error`, as Go's error string does. A handler's argument is decoded from the request's
first JSON parameter, as Go's jsonrpc decodes `params[0]`.

## Design notes

- **Byte-level codecs.** `ServerCodec` and `ClientCodec` move raw JSON text: the parameter list
  (`params`) and the result. The typed layer encodes and decodes with `argo`, so a codec never
  needs the argument types.
- **Go's grammar.** Unknown service and method names fail with Go's texts: `rpc: can't find service X`,
  `rpc: can't find method X`, and `rpc: service/method request ill-formed: X` for a name with no dot.
  A decode failure of a request header ends the connection, as in Go.
- **Concurrency.** `ServeCodec` runs each call in a task of one scope and serialises responses with a
  lane of one; the client's reader is a task the caller starts (`s.spawn(fn() ! { try c.Input() })`),
  because a handle cannot outlive its scope.

## Known gaps

- The default `net/rpc` wire is gob. The gob codec is not here: it needs #913, and `ServeCodec`
  takes any codec, so it plugs in later.
- `rpc.Server` has no `Accept` over a listener in the core package (it would need `wire`); the
  listener loop is `jsonrpc.Serve`.
