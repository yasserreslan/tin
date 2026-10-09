// Command asn1 is the Go side of tools/ci/asn1_check.tin: it reads one case per line on stdin and
// prints one line per case, for Go's encoding/asn1 (#913).
//
//	S <spec>   encode the value the spec describes: "E <hex>", or "ERR <message>"
//	D <hex>    read the elements in the bytes: "R <rendering>", or "ERR <message>"
//
// A spec is a list of tokens separated by spaces: i:<int64>, b:0|1, n (NULL), o:<arcs> (an OBJECT
// IDENTIFIER), x:<hex> (OCTET STRING), k:<bits>:<hex> (BIT STRING), u:<hex> (UTF8String), p:<hex>
// (PrintableString), a:<hex> (IA5String), m:<hex> (NumericString), t:<YYYY-MM-DDThh:mm:ss> (a time,
// UTCTime or GeneralizedTime as Go picks), r:<hex> (the bytes as they are), ( ) (SEQUENCE), [ ]
// (SET, in the order given), E<n>( ) (an EXPLICIT tag [n]), C<n>( ) (an IMPLICIT constructed tag [n]),
// and P<n>:<hex> (an IMPLICIT primitive tag [n]).
//
// A rendering prints each element as <class>.<tag>= followed by its value: bool:, int:, bits:<len>:<hex>,
// octets:<hex>, null, oid:<arcs>, str:<hex>, time:<UTC text>, or raw:<hex> for a tag Go's typed
// decoders do not name; a constructed element prints its children in parentheses.
package main

import (
	"bufio"
	"encoding/asn1"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Each spec leaf is marshaled as the one field of a struct with the tag Go's Marshal reads it with;
// the element is then the contents of the SEQUENCE it is wrapped in.
type intS struct{ V int64 }
type boolS struct{ V bool }
type octS struct{ V []byte }
type oidS struct{ V asn1.ObjectIdentifier }
type bitsS struct{ V asn1.BitString }
type utf8S struct {
	V string `asn1:"utf8"`
}
type printS struct {
	V string `asn1:"printable"`
}
type ia5S struct {
	V string `asn1:"ia5"`
}
type numS struct {
	V string `asn1:"numeric"`
}
type timeS struct{ V time.Time }

// element returns the one element Go marshals for v (a struct with one field).
func element(v any) ([]byte, error) {
	b, err := asn1.Marshal(v)
	if err != nil {
		return nil, err
	}
	var raw asn1.RawValue
	if _, err := asn1.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	return raw.Bytes, nil
}

// parser reads the tokens of a spec.
type parser struct {
	toks []string
	at   int
}

// list encodes the items up to the closing token (none for the top level).
func (p *parser) list(close string) ([]byte, error) {
	var out []byte
	for p.at < len(p.toks) {
		t := p.toks[p.at]
		p.at++
		if t == close {
			return out, nil
		}
		e, err := p.item(t)
		if err != nil {
			return nil, err
		}
		out = append(out, e...)
	}
	if close != "" {
		return nil, fmt.Errorf("unterminated %s", close)
	}
	return out, nil
}

// tagged reads the number between a prefix and a suffix of a token (E40( is "40" after "E").
func tagged(t string, prefix string) (int, bool) {
	n, err := strconv.Atoi(t[len(prefix) : len(t)-1])
	return n, err == nil
}

func (p *parser) item(t string) ([]byte, error) {
	switch {
	case t == "(":
		b, err := p.list(")")
		if err != nil {
			return nil, err
		}
		return asn1.Marshal(asn1.RawValue{Class: 0, Tag: 16, IsCompound: true, Bytes: b})
	case t == "[":
		b, err := p.list("]")
		if err != nil {
			return nil, err
		}
		return asn1.Marshal(asn1.RawValue{Class: 0, Tag: 17, IsCompound: true, Bytes: b})
	case strings.HasPrefix(t, "E") && strings.HasSuffix(t, "("):
		n, _ := tagged(t, "E")
		b, err := p.list(")")
		if err != nil {
			return nil, err
		}
		return asn1.Marshal(asn1.RawValue{Class: 2, Tag: n, IsCompound: true, Bytes: b})
	case strings.HasPrefix(t, "C") && strings.HasSuffix(t, "("):
		n, _ := tagged(t, "C")
		b, err := p.list(")")
		if err != nil {
			return nil, err
		}
		return asn1.Marshal(asn1.RawValue{Class: 2, Tag: n, IsCompound: true, Bytes: b})
	case strings.HasPrefix(t, "P"):
		k, v, _ := strings.Cut(t[1:], ":")
		n, _ := strconv.Atoi(k)
		raw, err := hex.DecodeString(v)
		if err != nil {
			return nil, err
		}
		return asn1.Marshal(asn1.RawValue{Class: 2, Tag: n, IsCompound: false, Bytes: raw})
	case t == "n":
		return asn1.Marshal(asn1.NullRawValue)
	}
	kind, v, _ := strings.Cut(t, ":")
	switch kind {
	case "i":
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, err
		}
		return element(intS{V: n})
	case "b":
		return element(boolS{V: v == "1"})
	case "o":
		var arcs asn1.ObjectIdentifier
		for _, part := range strings.Split(v, ".") {
			n, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("asn1: structure error: invalid object identifier")
			}
			arcs = append(arcs, n)
		}
		return element(oidS{V: arcs})
	case "x":
		raw, err := hex.DecodeString(v)
		if err != nil {
			return nil, err
		}
		return element(octS{V: raw})
	case "k":
		bits, h, _ := strings.Cut(v, ":")
		n, err := strconv.Atoi(bits)
		if err != nil {
			return nil, err
		}
		raw, err := hex.DecodeString(h)
		if err != nil {
			return nil, err
		}
		return element(bitsS{V: asn1.BitString{Bytes: raw, BitLength: n}})
	case "u", "p", "a", "m":
		raw, err := hex.DecodeString(v)
		if err != nil {
			return nil, err
		}
		s := string(raw)
		switch kind {
		case "u":
			return element(utf8S{V: s})
		case "p":
			return element(printS{V: s})
		case "a":
			return element(ia5S{V: s})
		}
		return element(numS{V: s})
	case "t":
		var y, mo, d, h, mi, s int
		if _, err := fmt.Sscanf(v, "%d-%d-%dT%d:%d:%d", &y, &mo, &d, &h, &mi, &s); err != nil {
			return nil, err
		}
		return element(timeS{V: time.Date(y, time.Month(mo), d, h, mi, s, 0, time.UTC)})
	case "r":
		raw, err := hex.DecodeString(v)
		if err != nil {
			return nil, err
		}
		return raw, nil
	}
	return nil, fmt.Errorf("unknown token %q", t)
}

// classify returns the error text the Tin side must match: Go's structure and syntax errors in full,
// and "other error" for the rest (time parsing, UTF-8 and internal messages Tin words differently).
func classify(err error) string {
	msg := err.Error()
	if (strings.HasPrefix(msg, "asn1: structure error: ") || strings.HasPrefix(msg, "asn1: syntax error: ")) && !strings.Contains(msg, "tags don't match") {
		return msg
	}
	return "other error"
}

// render prints the elements of b.
func render(b []byte) (string, error) {
	var parts []string
	for len(b) > 0 {
		var raw asn1.RawValue
		rest, err := asn1.Unmarshal(b, &raw)
		if err != nil {
			return "", err
		}
		s, err := renderRaw(raw)
		if err != nil {
			return "", err
		}
		parts = append(parts, s)
		b = rest
	}
	return strings.Join(parts, " "), nil
}

// renderRaw prints one element: its class and tag, then its value.
func renderRaw(r asn1.RawValue) (string, error) {
	label := fmt.Sprintf("%d.%d", r.Class, r.Tag)
	if r.IsCompound {
		inner, err := render(r.Bytes)
		if err != nil {
			return "", err
		}
		return label + "(" + inner + ")", nil
	}
	if r.Class != 0 {
		return label + "=raw:" + hex.EncodeToString(r.Bytes), nil
	}
	full := r.FullBytes
	switch r.Tag {
	case asn1.TagBoolean:
		var v bool
		if _, err := asn1.Unmarshal(full, &v); err != nil {
			return "", err
		}
		return label + fmt.Sprintf("=bool:%t", v), nil
	case asn1.TagInteger:
		var v int64
		if _, err := asn1.Unmarshal(full, &v); err != nil {
			return "", err
		}
		return label + fmt.Sprintf("=int:%d", v), nil
	case asn1.TagBitString:
		var v asn1.BitString
		if _, err := asn1.Unmarshal(full, &v); err != nil {
			return "", err
		}
		return label + fmt.Sprintf("=bits:%d:%s", v.BitLength, hex.EncodeToString(v.Bytes)), nil
	case asn1.TagOctetString:
		var v []byte
		if _, err := asn1.Unmarshal(full, &v); err != nil {
			return "", err
		}
		return label + "=octets:" + hex.EncodeToString(v), nil
	case asn1.TagNull:
		var v any
		if _, err := asn1.Unmarshal(full, &v); err != nil {
			return "", err
		}
		return label + "=null", nil
	case asn1.TagOID:
		var v asn1.ObjectIdentifier
		if _, err := asn1.Unmarshal(full, &v); err != nil {
			return "", err
		}
		parts := make([]string, len(v))
		for i, a := range v {
			parts[i] = strconv.Itoa(a)
		}
		return label + "=oid:" + strings.Join(parts, "."), nil
	case asn1.TagUTF8String, asn1.TagNumericString, asn1.TagPrintableString, asn1.TagT61String, asn1.TagIA5String:
		var v string
		if _, err := asn1.Unmarshal(full, &v); err != nil {
			return "", err
		}
		return label + "=str:" + hex.EncodeToString([]byte(v)), nil
	case asn1.TagUTCTime, asn1.TagGeneralizedTime:
		var v time.Time
		if _, err := asn1.Unmarshal(full, &v); err != nil {
			return "", err
		}
		v = v.UTC()
		return label + fmt.Sprintf("=time:%04d-%02d-%02dT%02d:%02d:%02dZ", v.Year(), int(v.Month()), v.Day(), v.Hour(), v.Minute(), v.Second()), nil
	}
	return label + "=raw:" + hex.EncodeToString(r.Bytes), nil
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		line := in.Text()
		if len(line) < 2 {
			continue
		}
		arg := line[2:]
		switch line[0] {
		case 'S':
			p := &parser{toks: strings.Fields(arg)}
			b, err := p.list("")
			if err != nil {
				fmt.Fprintf(out, "ERR %s\n", classify(err))
				continue
			}
			fmt.Fprintf(out, "E %s\n", hex.EncodeToString(b))
		case 'D':
			raw, err := hex.DecodeString(arg)
			if err != nil {
				fmt.Fprintf(out, "ERR %s\n", classify(err))
				continue
			}
			s, err := render(raw)
			if err != nil {
				fmt.Fprintf(out, "ERR %s\n", classify(err))
				continue
			}
			fmt.Fprintf(out, "R %s\n", s)
		}
	}
}
