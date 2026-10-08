// Command syslog sends a script of messages through Go's log/syslog and prints nothing on success. tools/ci/fixtures/syslog.tin runs
// the same script through syslog and tools/ci/syslog_check.tin compares the datagrams and stream bytes each one delivers (#929).
//
//	syslog NETWORK RADDR PRIORITY TAG   the arguments of syslog.Dial (RADDR "" with NETWORK "" is the local system log)
//
// The script comes on stdin, one operation per line:
//
//	S SEV HEX   the method for severity SEV (0 Emerg, 1 Alert, ..., 7 Debug) with the message HEX
//	W HEX       Write of the message HEX at the writer's priority
//	C           Close (the next message connects again)
//
// HEX is the message in hex, or "-" for an empty message.
package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"log/syslog"
	"os"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "usage: syslog NETWORK RADDR PRIORITY TAG")
		os.Exit(2)
	}
	prio, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Fprintln(os.Stderr, "priority:", err)
		os.Exit(2)
	}
	w, err := syslog.Dial(os.Args[1], os.Args[2], syslog.Priority(prio), os.Args[4])
	if err != nil {
		fmt.Fprintln(os.Stderr, "dial:", err)
		os.Exit(1)
	}
	methods := []func(string) error{w.Emerg, w.Alert, w.Crit, w.Err, w.Warning, w.Notice, w.Info, w.Debug}
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	for in.Scan() {
		fields := strings.Fields(in.Text())
		if len(fields) == 0 {
			continue
		}
		var msg []byte
		if fields[0] != "C" && fields[len(fields)-1] != "-" {
			msg, err = hex.DecodeString(fields[len(fields)-1])
			if err != nil {
				fmt.Fprintln(os.Stderr, "hex:", err)
				os.Exit(2)
			}
		}
		switch fields[0] {
		case "S":
			sev, err := strconv.Atoi(fields[1])
			if err != nil || sev < 0 || sev > 7 {
				fmt.Fprintln(os.Stderr, "severity:", fields[1])
				os.Exit(2)
			}
			err = methods[sev](string(msg))
			if err != nil {
				fmt.Fprintln(os.Stderr, "send:", err)
				os.Exit(1)
			}
		case "W":
			if _, err := w.Write(msg); err != nil {
				fmt.Fprintln(os.Stderr, "write:", err)
				os.Exit(1)
			}
		case "C":
			w.Close()
		default:
			fmt.Fprintln(os.Stderr, "unknown operation:", fields[0])
			os.Exit(2)
		}
	}
	if err := in.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "stdin:", err)
		os.Exit(1)
	}
}
