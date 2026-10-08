// Command netip reads a hex corpus on stdin and prints, for each case, what Go's net/netip says.
// tools/ci/fixtures/netip.tin prints the same lines and tools/ci/netip_check.tin compares them (#738).
// Every argument is x and the hex of its bytes; a string that does not parse stands for the zero value.
//
//	A <s>        ParseAddr: the text forms, the kind, the zone, every classification, Unmap, Next, Prev,
//	             the byte forms, the text and binary round trips and Prefix(bits) for the edge lengths
//	P <s>        ParsePrefix: String, Bits, IsValid, IsSingleIP, Addr, Masked and the text round trip
//	T <s>        ParseAddrPort: String, Port, Addr and the text round trip
//	C <a> <b>    Addr Compare, Less and ==
//	K <p> <a>    Prefix.Contains
//	O <p> <q>    Prefix Overlaps, Compare and ==
//	Q <x> <y>    AddrPort Compare and ==
//	W <a> <z>    Addr.WithZone
//	F <b>        AddrFromSlice and UnmarshalBinary of the bytes
//	R <a> <n>    PrefixFrom(a, n) with n decimal text
package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

var prefixBits = []int{-1, 0, 1, 7, 8, 9, 15, 16, 24, 31, 32, 33, 63, 64, 65, 96, 100, 127, 128, 129}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		fields := strings.Fields(in.Text())
		if len(fields) == 0 {
			continue
		}
		arg := func(i int) string {
			if i < len(fields) {
				b, err := hex.DecodeString(strings.TrimPrefix(fields[i], "x"))
				if err != nil {
					panic(err)
				}
				return string(b)
			}
			return ""
		}
		switch fields[0] {
		case "A":
			fmt.Fprintln(out, addrLine(arg(1)))
		case "P":
			fmt.Fprintln(out, prefixLine(arg(1)))
		case "T":
			fmt.Fprintln(out, addrPortLine(arg(1)))
		case "C":
			a, b := addrOr(arg(1)), addrOr(arg(2))
			fmt.Fprintf(out, "C %d %t %t %t\n", a.Compare(b), a.Less(b), b.Less(a), a == b)
		case "K":
			p, a := prefixOr(arg(1)), addrOr(arg(2))
			fmt.Fprintf(out, "K %t\n", p.Contains(a))
		case "O":
			p, q := prefixOr(arg(1)), prefixOr(arg(2))
			fmt.Fprintf(out, "O %t %t %d %t\n", p.Overlaps(q), q.Overlaps(p), p.Compare(q), p == q)
		case "Q":
			x, y := addrPortOr(arg(1)), addrPortOr(arg(2))
			fmt.Fprintf(out, "Q %d %t\n", x.Compare(y), x == y)
		case "W":
			a := addrOr(arg(1)).WithZone(arg(2))
			fmt.Fprintf(out, "W %q %q %t %q\n", a.String(), a.Zone(), a.IsUnspecified(), a.StringExpanded())
		case "F":
			b := []byte(arg(1))
			a, ok := netip.AddrFromSlice(b)
			var u netip.Addr
			err := u.UnmarshalBinary(b)
			fmt.Fprintf(out, "F %q %t %q %s\n", a.String(), ok, u.String(), faultText(err))
		case "R":
			n, err := strconv.Atoi(arg(2))
			if err != nil {
				panic(err)
			}
			p := netip.PrefixFrom(addrOr(arg(1)), n)
			fmt.Fprintf(out, "R %q %d %t %q %q\n", p.String(), p.Bits(), p.IsValid(), p.Addr().String(), p.Masked().String())
		default:
			panic("unknown op " + fields[0])
		}
	}
}

func faultText(err error) string {
	if err == nil {
		return "ok"
	}
	return strconv.Quote(err.Error())
}

func addrOr(s string) netip.Addr {
	a, _ := netip.ParseAddr(s)
	return a
}

func prefixOr(s string) netip.Prefix {
	p, _ := netip.ParsePrefix(s)
	return p
}

func addrPortOr(s string) netip.AddrPort {
	p, _ := netip.ParseAddrPort(s)
	return p
}

func bit(b bool) byte {
	if b {
		return '1'
	}
	return '0'
}

func as4(a netip.Addr) (s string) {
	defer func() {
		if r := recover(); r != nil {
			s = strconv.Quote(fmt.Sprint(r))
		}
	}()
	b := a.As4()
	return hex.EncodeToString(b[:])
}

func addrLine(s string) string {
	a, err := netip.ParseAddr(s)
	if err != nil {
		return "A err " + strconv.Quote(err.Error())
	}
	flags := []byte{bit(a.IsValid()), bit(a.Is4()), bit(a.Is6()), bit(a.Is4In6()), bit(a.IsLoopback()), bit(a.IsPrivate()),
		bit(a.IsUnspecified()), bit(a.IsMulticast()), bit(a.IsLinkLocalUnicast()), bit(a.IsLinkLocalMulticast()),
		bit(a.IsInterfaceLocalMulticast()), bit(a.IsGlobalUnicast())}
	a16 := a.As16()
	text, _ := a.MarshalText()
	bin, _ := a.MarshalBinary()
	var t, b netip.Addr
	terr := t.UnmarshalText(text)
	berr := b.UnmarshalBinary(bin)
	var sb strings.Builder
	fmt.Fprintf(&sb, "A %q %q %d %s %q unmap=%q next=%q prev=%q slice=%x as16=%x as4=%s bin=%x text=%q rt=%t,%t,%t,%t",
		a.String(), a.StringExpanded(), a.BitLen(), flags, a.Zone(), a.Unmap().String(), a.Next().String(), a.Prev().String(),
		a.AsSlice(), a16[:], as4(a), bin, text, terr == nil, t == a, berr == nil, b == a)
	fmt.Fprintf(&sb, " np=%t pn=%t", a.Next().Prev() == a, a.Prev().Next() == a)
	for _, n := range prefixBits {
		p, err := a.Prefix(n)
		if err != nil {
			fmt.Fprintf(&sb, " /%d:%q", n, err.Error())
		} else {
			fmt.Fprintf(&sb, " /%d:%q", n, p.String())
		}
	}
	return sb.String()
}

func prefixLine(s string) string {
	p, err := netip.ParsePrefix(s)
	if err != nil {
		return "P err " + strconv.Quote(err.Error())
	}
	text, _ := p.MarshalText()
	var t netip.Prefix
	terr := t.UnmarshalText(text)
	return fmt.Sprintf("P %q %d %t %t %q %q text=%q rt=%t,%t", p.String(), p.Bits(), p.IsValid(), p.IsSingleIP(),
		p.Addr().String(), p.Masked().String(), text, terr == nil, t == p)
}

func addrPortLine(s string) string {
	p, err := netip.ParseAddrPort(s)
	if err != nil {
		return "T err " + strconv.Quote(err.Error())
	}
	text, _ := p.MarshalText()
	var t netip.AddrPort
	terr := t.UnmarshalText(text)
	return fmt.Sprintf("T %q %d %q %t text=%q rt=%t,%t", p.String(), p.Port(), p.Addr().String(), p.IsValid(), text, terr == nil, t == p)
}
