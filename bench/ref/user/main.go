package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"os/user"
)

func main() {
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		f := strings.Split(s.Text(), "\t")
		if len(f) == 0 { continue }
		var err error
		switch f[0] {
		case "C":
			var u *user.User
			u, err = user.Current()
			if err == nil { printUser(u) }
		case "I":
			var u *user.User
			u, err = user.LookupId(f[1])
			if err == nil { printUser(u) }
		case "N":
			var u *user.User
			u, err = user.Lookup(f[1])
			if err == nil { printUser(u) }
		case "G":
			var g *user.Group
			g, err = user.LookupGroup(f[1])
			if err == nil { fmt.Printf("G\t%s\t%s\n", g.Name, g.Gid) }
		case "M":
			var u *user.User
			u, err = user.LookupId(f[1])
			if err == nil {
				var ids []string
				ids, err = u.GroupIds()
				if err == nil { fmt.Printf("M\t%s\n", strings.Join(ids, ",")) }
			}
		}
		if err != nil { fmt.Printf("E\t%s\n", err) }
	}
}

func printUser(u *user.User) {
	fmt.Printf("U\t%s\t%s\t%s\t%s\t%s\n", u.Username, u.Uid, u.Gid, u.Name, u.HomeDir)
}
