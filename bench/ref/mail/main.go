// Command mail reads a small text protocol corpus and prints Go net/mail and net/textproto results.
package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"net/mail"
	"net/textproto"
	"os"
	"strings"
)

func main() {
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		f := strings.Fields(s.Text())
		if len(f) < 2 {
			continue
		}
		v, err := hex.DecodeString(f[1])
		if err != nil {
			panic(err)
		}
		x := string(v)
		switch f[0] {
		case "A":
			a, err := mail.ParseAddress(x)
			if err != nil {
				fmt.Println("A ERR")
			} else {
				fmt.Printf("A %q %q\n", a.Name, a.Address)
			}
		case "L":
			aa, err := mail.ParseAddressList(x)
			if err != nil {
				fmt.Println("L ERR")
				continue
			}
			fmt.Print("L")
			for _, a := range aa {
				fmt.Printf(" %q %q", a.Name, a.Address)
			}
			fmt.Println()
		case "D":
			d, err := mail.ParseDate(x)
			if err != nil {
				fmt.Println("D ERR")
			} else {
				fmt.Printf("D %d\n", d.UnixNano())
			}
		case "C":
			fmt.Printf("C %q\n", textproto.CanonicalMIMEHeaderKey(x))
		case "T":
			r := textproto.NewReader(bufio.NewReader(bytes.NewReader(v)))
			b, err := r.ReadDotBytes()
			if err != nil {
				fmt.Println("T ERR")
			} else {
				fmt.Printf("T %q\n", string(b))
			}
		case "F":
			r := textproto.NewReader(bufio.NewReader(bytes.NewReader(v)))
			line, err := r.ReadContinuedLine()
			if err != nil {
				fmt.Println("F ERR")
			} else {
				fmt.Printf("F %q\n", line)
			}
		case "M":
			m, err := mail.ReadMessage(bytes.NewReader(v))
			if err != nil {
				fmt.Println("M ERR")
			} else {
				body, _ := io.ReadAll(m.Body)
				fmt.Printf("M %q %q %q\n", m.Header.Get("Subject"), m.Header.Get("From"), string(body))
			}
		}
	}
}
