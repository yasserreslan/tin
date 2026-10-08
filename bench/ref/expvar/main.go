// Command expvar publishes with Go's expvar the variables that tools/ci/fixtures/expvar.tin
// publishes, serves them through net/http, and prints what tools/ci/expvar_check.tin compares
// (#923): the Content-Type line, then the body without cmdline and memstats, which expvar
// registers by itself (a Tin program has no such variables).
package main

import (
	"expvar"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
)

// hits is how many GET /hit requests the Tin fixture serves before the check reads the variables.
const hits = 200

func main() {
	requests := expvar.NewInt("requests")
	for i := 0; i < hits; i++ {
		requests.Add(1)
	}
	errs := expvar.NewInt("errors")
	errs.Set(-7)
	expvar.Publish("errors_alias", errs)
	wrap := expvar.NewInt("wrap")
	wrap.Set(math.MaxInt64)
	wrap.Add(1)

	ratio := expvar.NewFloat("ratio")
	ratio.Set(0.1)
	load := expvar.NewFloat("load")
	load.Add(0.5)
	load.Add(0.5)
	load.Add(0.5)
	expvar.NewFloat("big").Set(1e21)
	expvar.NewFloat("tiny").Set(0.00000015)
	expvar.NewFloat("inf").Set(math.Inf(1))
	expvar.NewFloat("nan").Set(math.NaN())

	build := expvar.NewString("build")
	build.Set("tin \"1\"\n\t<&>  é 日本 \x01 \\ end")
	expvar.NewString("invalid").Set("\xff\xfe ok")
	expvar.NewString("empty").Set("")

	// Go's expvar has no Bool: a flag is published as a Func, which renders its JSON value. A Func
	// runs each time its variable is served, so ready (set after it is published) is served true.
	ready := false
	expvar.Publish("ready", expvar.Func(func() any { return ready }))
	ready = true
	flag := false
	expvar.Publish("flag", expvar.Func(func() any { return flag }))
	expvar.Publish("hits", expvar.Func(func() any { return requests.Value() }))
	// Func values are encoded as json.Marshal writes them: HTML characters and \b \f in strings,
	// a fraction, and a NaN, which has no JSON form and so reads as nothing.
	expvar.Publish("label", expvar.Func(func() any { return "x<y & \"z\"\b\f" }))
	expvar.Publish("scale", expvar.Func(func() any { return 1234567.5 }))
	expvar.Publish("gap", expvar.Func(func() any { return math.NaN() }))

	h := expvar.NewMap("http")
	h.Add("ok", 120)
	h.Add("err", 3)
	h.AddFloat("latency", 0.25)
	h.AddFloat("latency", 0.5)
	node := new(expvar.String)
	node.Set("a-1")
	h.Set("node", node)
	h.Set("up", expvar.Func(func() any { return true }))
	h.Set("requests", requests)
	h.Add("err", 1)

	expvar.NewMap("nomap")
	gone := expvar.NewMap("gone")
	gone.Add("x", 1)
	gone.Delete("x")
	reset := expvar.NewMap("reset")
	reset.Add("a", 1)
	reset.Init()
	reset.Add("b", 2)

	srv := httptest.NewServer(expvar.Handler())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/debug/vars")
	if err != nil {
		fmt.Fprintln(os.Stderr, "expvar:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "expvar:", err)
		os.Exit(1)
	}
	fmt.Printf("content-type: %s\n", resp.Header.Get("Content-Type"))
	fmt.Print(withoutRuntimeVars(string(body)))
}

// withoutRuntimeVars removes the cmdline and memstats lines from Go's handler output and joins
// the rest in its layout: "{", one "name": value line per variable separated by ",", "}".
func withoutRuntimeVars(body string) string {
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	var entries []string
	for _, line := range lines[1 : len(lines)-1] {
		line = strings.TrimSuffix(line, ",")
		if line == "" || strings.HasPrefix(line, `"cmdline": `) || strings.HasPrefix(line, `"memstats": `) {
			continue
		}
		entries = append(entries, line)
	}
	return "{\n" + strings.Join(entries, ",\n") + "\n}\n"
}
