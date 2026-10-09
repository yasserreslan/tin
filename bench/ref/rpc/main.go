// Command main is Go's net/rpc and net/rpc/jsonrpc running the calls of toolchain/tests/v2/rpc.tin, printing the same lines: the
// strict test's output is diffed against this program's output.
package main

import (
	"errors"
	"fmt"
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"
)

// Args and Reply are the typed parameters of the Arith service.
type Args struct {
	A int
	B int
}

type Reply struct {
	C int
}

// Named is a struct with a slice field.
type Named struct {
	Name string
	Tags []string
}

type Arith struct{}

func (Arith) Multiply(a *Args, r *Reply) error {
	r.C = a.A * a.B
	return nil
}

func (Arith) Divide(a *Args, r *Reply) error {
	if a.B == 0 {
		return errors.New("divide by zero")
	}
	r.C = a.A / a.B
	return nil
}

type Math struct{}

func (Math) Double(n *int, r *int) error {
	*r = *n * 2
	return nil
}

type Text struct{}

func (Text) Echo(s *string, r *string) error {
	*r = "echo " + *s
	return nil
}

func (Text) Angry(s *string, r *string) error {
	return errors.New("<no> & more")
}

func (Text) Ctl(s *string, r *string) error {
	return errors.New(string([]byte{8, 12, 1, 34, 92, 10, 9, 60, 62, 38, 0xE2, 0x80, 0xA8}))
}

type Stats struct{}

func (Stats) Sum(xs *[]int, r *int) error {
	total := 0
	for _, x := range *xs {
		total += x
	}
	*r = total
	return nil
}

type Users struct{}

func (Users) Tag(n *Named, r *Named) error {
	tags := append([]string{}, n.Tags...)
	tags = append(tags, "seen")
	r.Name = n.Name
	r.Tags = tags
	return nil
}

// duplex is one end of two io pipes: it reads from r and writes to w.
type duplex struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func (d duplex) Read(p []byte) (int, error) { return d.r.Read(p) }

func (d duplex) Write(p []byte) (int, error) { return d.w.Write(p) }

func (d duplex) Close() error {
	d.r.Close()
	return d.w.Close()
}

func setup() *rpc.Server {
	srv := rpc.NewServer()
	must(srv.RegisterName("Arith", Arith{}))
	must(srv.RegisterName("Math", Math{}))
	must(srv.RegisterName("Text", Text{}))
	must(srv.RegisterName("Stats", Stats{}))
	must(srv.RegisterName("Users", Users{}))
	return srv
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// report prints a call's fault as the Tin test does.
func report(label string, err error) {
	fmt.Println(label, "error:", err)
}

func session(srv *rpc.Server) {
	rq, wq := io.Pipe()
	rr, wr := io.Pipe()
	go srv.ServeCodec(jsonrpc.NewServerCodec(duplex{rq, wr}))
	client := rpc.NewClientWithCodec(jsonrpc.NewClientCodec(duplex{rr, wq}))

	m := Reply{C: -1}
	if err := client.Call("Arith.Multiply", Args{A: 7, B: 6}, &m); err != nil {
		report("multiply", err)
		m.C = -1
	}
	fmt.Println("multiply", m.C)

	d := Reply{C: -1}
	if err := client.Call("Arith.Divide", Args{A: 7, B: 2}, &d); err != nil {
		report("divide", err)
		d.C = -1
	}
	fmt.Println("divide", d.C)

	z := Reply{C: -1}
	if err := client.Call("Arith.Divide", Args{A: 7, B: 0}, &z); err != nil {
		report("divide by zero", err)
		z.C = -1
	}
	fmt.Println("divide by zero", z.C)

	dbl := -1
	if err := client.Call("Math.Double", 21, &dbl); err != nil {
		report("double", err)
		dbl = -1
	}
	fmt.Println("double", dbl)

	e := ""
	if err := client.Call("Text.Echo", "hi", &e); err != nil {
		report("echo", err)
		e = ""
	}
	fmt.Println("echo", e)

	total := -1
	if err := client.Call("Stats.Sum", []int{1, 2, 3, 4}, &total); err != nil {
		report("sum", err)
		total = -1
	}
	fmt.Println("sum", total)

	named := Named{Name: "", Tags: []string{}}
	if err := client.Call("Users.Tag", Named{Name: "ada", Tags: []string{"x"}}, &named); err != nil {
		report("tag", err)
		named = Named{Name: "", Tags: []string{}}
	}
	fmt.Println("tag", named.Name, named.Tags)

	angry := ""
	if err := client.Call("Text.Angry", "x", &angry); err != nil {
		report("angry", err)
		angry = ""
	}
	fmt.Println("angry", angry)

	nope := Reply{C: -1}
	if err := client.Call("Arith.Nope", Args{A: 1, B: 1}, &nope); err != nil {
		report("unknown method", err)
		nope.C = -1
	}
	fmt.Println("unknown method", nope.C)

	nosvc := Reply{C: -1}
	if err := client.Call("Nope.Call", Args{A: 1, B: 1}, &nosvc); err != nil {
		report("unknown service", err)
		nosvc.C = -1
	}
	fmt.Println("unknown service", nosvc.C)

	bad := Reply{C: -1}
	if err := client.Call("Nope", Args{A: 1, B: 1}, &bad); err != nil {
		report("ill-formed", err)
		bad.C = -1
	}
	fmt.Println("ill-formed", bad.C)

	// Calls in flight at once: each Go sends its request, and the replies are taken in order.
	r1, r2, r3 := Reply{C: -1}, Reply{C: -1}, Reply{C: -1}
	c1 := client.Go("Arith.Multiply", Args{A: 2, B: 3}, &r1, nil)
	c2 := client.Go("Arith.Multiply", Args{A: 4, B: 5}, &r2, nil)
	c3 := client.Go("Arith.Multiply", Args{A: 6, B: 7}, &r3, nil)
	<-c3.Done
	if c3.Error != nil {
		report("pipelined 3", c3.Error)
		r3.C = -1
	}
	<-c1.Done
	if c1.Error != nil {
		report("pipelined 1", c1.Error)
		r1.C = -1
	}
	<-c2.Done
	if c2.Error != nil {
		report("pipelined 2", c2.Error)
		r2.C = -1
	}
	fmt.Println("pipelined", r1.C, r2.C, r3.C)

	if err := client.Close(); err != nil {
		report("close", err)
	}
	late := Reply{C: -1}
	if err := client.Call("Arith.Multiply", Args{A: 1, B: 1}, &late); err != nil {
		fmt.Println("after close:", err, errors.Is(err, rpc.ErrShutdown))
		late.C = -1
	}
	fmt.Println("after close", late.C)
}

// raw sends the request bytes to a server and prints the response bytes it writes.
func raw(srv *rpc.Server, request string) {
	rq, wq := io.Pipe()
	rr, wr := io.Pipe()
	go srv.ServeCodec(jsonrpc.NewServerCodec(duplex{rq, wr}))
	if _, err := wq.Write([]byte(request)); err != nil {
		fmt.Println("raw failed:", err)
		return
	}
	wq.Close()
	out, err := io.ReadAll(rr)
	if err != nil {
		fmt.Println("raw failed:", err)
		return
	}
	fmt.Println(string(out))
}

func main() {
	srv := setup()
	session(srv)
	raw(srv, `{"method":"Arith.Multiply","params":[{"A":7,"B":6}],"id":5}`)
	raw(srv, `{"method":"Arith.Nope","params":[{}],"id":"abc"}`)
	raw(srv, `{"method":"Text.Angry","params":["x"],"id":7}`)
	raw(srv, `{"method":"Arith.Multiply","id":8}`)
	raw(srv, `{"method":"Text.Ctl","params":["x"],"id":9}`)
}
