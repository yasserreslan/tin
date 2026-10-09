// Command rpc_twin is Go's net/rpc and net/rpc/jsonrpc for the Go twin check: `serve ADDR` serves the same methods as
// tools/ci/fixtures/rpc_twin.tin over TCP, and `call ADDR` runs the same session against a server on ADDR and prints the same
// lines.
package main

import (
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
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

func setup() *rpc.Server {
	srv := rpc.NewServer()
	for name, svc := range map[string]any{"Arith": Arith{}, "Math": Math{}, "Text": Text{}, "Stats": Stats{}, "Users": Users{}} {
		if err := srv.RegisterName(name, svc); err != nil {
			panic(err)
		}
	}
	return srv
}

// report prints a call's fault as the Tin side prints it.
func report(label string, err error) {
	fmt.Println(label, "error:", err)
}

// session makes the calls of the Tin twin and prints each outcome.
func session(client *rpc.Client) {
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

func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage: rpc_twin serve|call ADDR")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		ln, err := net.Listen("tcp", os.Args[2])
		if err != nil {
			fmt.Println("serve failed:", err)
			os.Exit(1)
		}
		srv := setup()
		for {
			conn, err := ln.Accept()
			if err != nil {
				fmt.Println("accept failed:", err)
				os.Exit(1)
			}
			go srv.ServeCodec(jsonrpc.NewServerCodec(conn))
		}
	case "call":
		conn, err := net.Dial("tcp", os.Args[2])
		if err != nil {
			fmt.Println("call failed:", err)
			os.Exit(1)
		}
		session(jsonrpc.NewClient(conn))
	default:
		fmt.Println("usage: rpc_twin serve|call ADDR")
		os.Exit(2)
	}
}
