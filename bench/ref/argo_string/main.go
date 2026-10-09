// The argo string twin, Go side: read one hex-encoded string per line on stdin and print the hex of encoding/json's Marshal of it,
// so tools/ci/argo_string_check.tin can compare it with argo.Str (#943).
package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		raw, err := hex.DecodeString(in.Text())
		if err != nil {
			fmt.Fprintln(out, "bad input:", err)
			continue
		}
		b, err := json.Marshal(string(raw))
		if err != nil {
			fmt.Fprintln(out, "error:", err)
			continue
		}
		fmt.Fprintln(out, hex.EncodeToString(b))
	}
}
