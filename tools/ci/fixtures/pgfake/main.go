// The fake PostgreSQL server of tools/ci/postgres_check.tin: protocol 3.0 with SCRAM-SHA-256, MD5 and cleartext authentication, typed
// binary bindings, transactions, a statement cache, and hostile peers (fragmented, coalesced and malformed frames) chosen by the user
// name. A control server on $PORT makes and inspects servers:
//
//	GET /new?tls=0|1       a new server on a free port (tls=1: TLS 1.3 with the certificate and key named by the arguments); answers
//	                       its port
//	GET /stats?port=P      "auths=N parses=N closes=N maxstatements=N open=N peak=N errors=N"
//	GET /errors?port=P     the protocol violations the clients committed (the fake's own assertions), one per line
//	GET /resetpeak?port=P  the peak of simultaneous connections starts again from the open ones
//	GET /drop?port=P       closes every connection
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

func i16(n int) []byte { return []byte{byte(n >> 8), byte(n)} }
func i32(n uint32) []byte {
	return []byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
}
func cstr(s string) []byte { return append([]byte(s), 0) }

func frame(kind byte, payload []byte) []byte {
	out := append([]byte{kind}, i32(uint32(len(payload)+4))...)
	return append(out, payload...)
}

func mac(key, msg []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(msg)
	return h.Sum(nil)
}

func xor(a, b []byte) []byte {
	out := make([]byte, len(a))
	for i := range a {
		out[i] = a[i] ^ b[i]
	}
	return out
}

func pbkdf2(password, salt []byte, iter int) []byte {
	u := mac(password, append(append([]byte{}, salt...), 0, 0, 0, 1))
	out := append([]byte{}, u...)
	for i := 1; i < iter; i++ {
		u = mac(password, u)
		for j := range out {
			out[j] ^= u[j]
		}
	}
	return out
}

// assertion is a protocol violation by the client; connErr is a broken connection, which is not one.
type assertion string
type connErr struct{ error }

func must(ok bool, msg string) {
	if !ok {
		panic(assertion(msg))
	}
}

type reader struct {
	data []byte
	at   int
}

func (r *reader) take(n int) []byte {
	must(0 <= n && n <= len(r.data)-r.at, "truncated client frame")
	out := r.data[r.at : r.at+n]
	r.at += n
	return out
}
func (r *reader) number(n int) int {
	v := 0
	for _, b := range r.take(n) {
		v = v<<8 | int(b)
	}
	return v
}
func (r *reader) str() []byte {
	end := bytes.IndexByte(r.data[r.at:], 0)
	must(end >= 0, "unterminated client string")
	return r.take(end + 1)[:end]
}
func (r *reader) done() { must(r.at == len(r.data), "trailing client frame bytes") }

type fake struct {
	tlsCfg *tls.Config

	lock          sync.Mutex
	users         map[int64][]byte
	nextID        int64
	conns         []net.Conn
	errors        []string
	parses        int
	closes        int
	auths         int
	maxStatements int
	open, peak    int
}

type stmt struct {
	sql  []byte
	oids []int
}

type session struct {
	f          *fake
	s          net.Conn
	statements map[string]stmt
	status     byte
	staged     map[int64][]byte
	discard    bool
	sql        []byte
	vals       []any
	oids       []int
	user       string
}

func (s *session) receive(n int) []byte {
	s.s.SetReadDeadline(time.Now().Add(10 * time.Second))
	b := make([]byte, n)
	if _, err := io.ReadFull(s.s, b); err != nil {
		panic(connErr{err})
	}
	return b
}

func (s *session) write(b []byte) {
	if _, err := s.s.Write(b); err != nil {
		panic(connErr{err})
	}
}

func (s *session) message() (byte, *reader) {
	kind := s.receive(1)[0]
	n := int(binary.BigEndian.Uint32(s.receive(4)))
	must(4 <= n && n <= 16777216, "message length")
	return kind, &reader{data: s.receive(n - 4)}
}

func (s *session) send(kind byte, payload []byte) { s.write(frame(kind, payload)) }

func (s *session) errorMsg(state, message, detail string) {
	fields := append([]byte("SERROR\x00VERROR\x00C"), cstr(state)...)
	fields = append(fields, 'M')
	fields = append(fields, cstr(message)...)
	if detail != "" {
		fields = append(fields, 'D')
		fields = append(fields, cstr(detail)...)
	}
	s.send('E', append(fields, 0))
	if s.status == 'T' {
		s.status = 'E'
	}
}

func (s *session) ready() { s.send('Z', []byte{s.status}) }

func (s *session) auth() bool {
	n := int(binary.BigEndian.Uint32(s.receive(4)))
	body := s.receive(n - 4)
	if n == 8 && binary.BigEndian.Uint32(body) == 80877103 {
		// SSLRequest: S and TLS when this server has a certificate, N otherwise.
		if s.f.tlsCfg == nil {
			s.write([]byte("N"))
			return false
		}
		s.write([]byte("S"))
		t := tls.Server(s.s, s.f.tlsCfg)
		if err := t.Handshake(); err != nil {
			panic(connErr{err})
		}
		s.s = t
		n = int(binary.BigEndian.Uint32(s.receive(4)))
		body = s.receive(n - 4)
	} else if s.f.tlsCfg != nil {
		s.errorMsg("28000", "no pg_hba.conf entry for host, no encryption", "")
		return false
	}
	r := &reader{data: body}
	must(r.number(4) == 196608, "protocol must be 3.0")
	opts := map[string]string{}
	for {
		key := string(r.str())
		if key == "" {
			break
		}
		opts[key] = string(r.str())
	}
	r.done()
	s.user = opts["user"]
	must(opts["database"] == "tin" && opts["client_encoding"] == "UTF8", "startup options")
	switch s.user {
	case "tls":
		s.errorMsg("28000", "no pg_hba.conf entry for host, no encryption", "")
		return false
	case "premature":
		s.ready()
		return false
	case "unknown":
		s.send('R', i32(7))
		return false
	}
	password := []byte("tinpass")
	switch s.user {
	case "unicode":
		password = []byte("IX é 각") // input has soft hyphen, decomposed accents/Hangul
	case "prohibited":
		password = []byte("\x07ª") // PostgreSQL falls back to the original bytes
	}
	if s.user == "md5" || s.user == "clear" {
		salt := []byte{0x00, 0xff, 0x80, 'x'}
		if s.user == "md5" {
			s.send('R', append(i32(5), salt...))
		} else {
			s.send('R', i32(3))
		}
		kind, r := s.message()
		must(kind == 'p', "password message")
		got := r.str()
		r.done()
		want := password
		if s.user == "md5" {
			inner := md5.Sum(append(append([]byte{}, password...), s.user...))
			innerHex := hex.EncodeToString(inner[:])
			outer := md5.Sum(append([]byte(innerHex), salt...))
			want = []byte("md5" + hex.EncodeToString(outer[:]))
		}
		if !bytes.Equal(got, want) {
			s.errorMsg("28P01", "password authentication failed", "")
			return false
		}
	} else {
		// Split the tag/length across TCP reads, testing the client's framing.
		msg := frame('R', append(i32(10), []byte("SCRAM-SHA-256\x00\x00")...))
		s.write(msg[:2])
		time.Sleep(2 * time.Millisecond)
		s.write(msg[2:])
		kind, r := s.message()
		must(kind == 'p' && string(r.str()) == "SCRAM-SHA-256", "SASLInitialResponse")
		first := r.take(r.number(4))
		r.done()
		must(bytes.HasPrefix(first, []byte("n,,n=,r=")), "client-first-message")
		bare := first[3:]
		nonce := bytes.SplitN(bare, []byte("r="), 2)[1]
		serverNonce := append(append([]byte{}, nonce...), "SERVERnonce"...)
		if s.user == "badnonce" {
			serverNonce = []byte("WRONGnonce")
		}
		salt := []byte("tin-test-salt\x00\xff")
		serverFirst := []byte("r=" + string(serverNonce) + ",s=" + base64.StdEncoding.EncodeToString(salt) + ",i=4096")
		switch s.user {
		case "duplicate":
			serverFirst = append(serverFirst, ",r=duplicate"...)
		case "slowauth":
			serverFirst = bytes.Replace(serverFirst, []byte("i=4096"), []byte("i=1000000"), 1)
		case "badcount":
			serverFirst = bytes.Replace(serverFirst, []byte("i=4096"), []byte("i=1000001"), 1)
		}
		s.send('R', append(i32(11), serverFirst...))
		kind, r = s.message()
		must(kind == 'p', "SASLResponse")
		final := r.take(len(r.data))
		r.done()
		at := bytes.LastIndex(final, []byte(",p="))
		must(at >= 0, "client-final-message proof")
		withoutProof, proof := final[:at], final[at+3:]
		must(string(withoutProof) == "c=biws,r="+string(serverNonce), "client-final-message")
		authMessage := append(append(append(append([]byte{}, bare...), ','), serverFirst...), append([]byte{','}, withoutProof...)...)
		salted := pbkdf2(password, salt, 4096)
		clientKey := mac(salted, []byte("Client Key"))
		stored := sha256.Sum256(clientKey)
		want := xor(clientKey, mac(stored[:], authMessage))
		got, _ := base64.StdEncoding.DecodeString(string(proof))
		if subtle.ConstantTimeCompare(want, got) != 1 {
			s.errorMsg("28P01", "password authentication failed", "")
			return false
		}
		verifier := mac(mac(salted, []byte("Server Key")), authMessage)
		if s.user == "badverifier" {
			verifier = make([]byte, 32)
		}
		s.send('R', append(i32(12), []byte("v="+base64.StdEncoding.EncodeToString(verifier))...))
	}
	s.f.lock.Lock()
	s.f.auths++
	s.f.lock.Unlock()
	// Coalesce authentication and metadata; unknown Error/Notice fields are legal.
	var out []byte
	out = append(out, frame('R', i32(0))...)
	out = append(out, frame('S', []byte("server_version\x0015.0\x00"))...)
	out = append(out, frame('K', append(i32(1234), i32(5678)...))...)
	out = append(out, frame('N', []byte("SNOTICE\x00C00000\x00Mhello\x00Ddetail\x00Xunknown\x00\x00"))...)
	out = append(out, frame('Z', []byte("I"))...)
	s.write(out)
	return true
}

type col struct {
	name string
	oid  int
}

func (s *session) columns() []col {
	sql := string(s.sql)
	switch {
	case strings.HasPrefix(sql, "SELECT 1 /"):
		return []col{{"value", 20}}
	case strings.HasPrefix(sql, "INSERT"):
		return []col{{"id", 20}}
	case strings.HasPrefix(sql, "SELECT id, name"):
		return []col{{"id", 20}, {"name", 25}}
	case sql == "SELECT COUNT(*) FROM users":
		return []col{{"count", 20}}
	case strings.HasPrefix(sql, "SELECT pg_sleep"):
		return []col{{"pg_sleep", 25}}
	case strings.HasPrefix(sql, "SELECT $1 AS i,"):
		names := []string{"i", "f", "s", "b", "bytes"}
		var out []col
		for i, n := range names {
			out = append(out, col{n, s.oids[i]})
		}
		return out
	case strings.HasPrefix(sql, "SELECT -123::"):
		names := []string{"small", "medium", "large", "real", "double", "yes", "no", "empty", "num", "s", "day", "ts", "tz", "uid", "j", "jb"}
		oids := []int{21, 23, 20, 700, 701, 16, 16, 25, 1700, 1043, 1082, 1114, 1184, 2950, 114, 3802}
		var out []col
		for i, n := range names {
			out = append(out, col{n, oids[i]})
		}
		return out
	case strings.HasPrefix(sql, "SELECT $1 AS value"), strings.HasPrefix(sql, "SELECT $1 AS f"):
		return []col{{"value", s.oids[0]}}
	}
	return nil
}

func (s *session) describe(cols []col) []byte {
	data := i16(len(cols))
	for _, c := range cols {
		data = append(data, cstr(c.name)...)
		data = append(data, i32(0)...)
		data = append(data, i16(0)...)
		data = append(data, i32(uint32(c.oid))...)
		data = append(data, i16(0xffff)...)
		data = append(data, i32(0xffffffff)...)
		data = append(data, i16(0)...)
	}
	return frame('T', data)
}

// text is how Python's str() wrote a value: integers in decimal, floats shortest-round-trip, bytes as they are.
func text(v any) []byte {
	switch x := v.(type) {
	case nil:
		return nil
	case []byte:
		return x
	case string:
		return []byte(x)
	case int64:
		return []byte(strconv.FormatInt(x, 10))
	case int:
		return []byte(strconv.Itoa(x))
	case float64:
		return []byte(strconv.FormatFloat(x, 'g', -1, 64))
	case bool:
		if x {
			return []byte("True")
		}
		return []byte("False")
	}
	panic(fmt.Sprint("text of ", v))
}

func (s *session) rows(cols []col, rows [][]any, tag string, described bool) {
	var data []byte
	if !described {
		data = s.describe(cols)
	}
	for _, row := range rows {
		payload := i16(len(row))
		for _, v := range row {
			if v == nil {
				payload = append(payload, i32(0xffffffff)...)
			} else {
				b := text(v)
				payload = append(payload, i32(uint32(len(b)))...)
				payload = append(payload, b...)
			}
		}
		data = append(data, frame('D', payload)...)
	}
	s.write(append(data, frame('C', cstr(tag))...))
}

func asInt(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case float64:
		return int64(x)
	}
	return 0
}

func asFloat(v any) float64 {
	switch x := v.(type) {
	case int64:
		return float64(x)
	case float64:
		return x
	}
	return 0
}

func (s *session) execute(described bool) {
	sql, vals := string(s.sql), s.vals
	if s.status == 'E' && sql != "ROLLBACK" && sql != "COMMIT" {
		s.errorMsg("25P02", "current transaction is aborted", "")
		return
	}
	switch {
	case sql == "BEGIN":
		must(s.status == 'I', "BEGIN inside a transaction")
		s.status = 'T'
		s.send('C', cstr("BEGIN"))
		return
	case sql == "COMMIT" || sql == "ROLLBACK":
		if sql == "COMMIT" && s.status == 'T' {
			s.f.lock.Lock()
			for k, v := range s.staged {
				s.f.users[k] = v
			}
			s.f.lock.Unlock()
		}
		s.staged = map[int64][]byte{}
		s.status = 'I'
		s.send('C', cstr(sql))
		return
	case strings.HasPrefix(sql, "CREATE TABLE"):
		s.send('C', cstr("CREATE TABLE"))
		return
	case strings.Contains(sql, "FROM nope"):
		s.errorMsg("42P01", "relation \"nope\" does not exist", "test detail")
		return
	case strings.HasPrefix(sql, "SELECT 1 /"):
		d := asInt(vals[0])
		if d == 0 {
			s.errorMsg("22012", "division by zero", "")
		} else {
			s.rows(s.columns(), [][]any{{int64(1) / d}}, "SELECT 1", described)
		}
		return
	case strings.HasPrefix(sql, "UPDATE users"):
		s.send('C', cstr("UPDATE 0"))
		return
	case strings.HasPrefix(sql, "INSERT"):
		s.f.lock.Lock()
		uid := s.f.nextID
		s.f.nextID++
		name := vals[0].([]byte)
		if s.status == 'T' {
			s.staged[uid] = name
		} else {
			s.f.users[uid] = name
		}
		s.f.lock.Unlock()
		s.rows(s.columns(), [][]any{{uid}}, "INSERT 0 1", described)
		return
	case strings.HasPrefix(sql, "SELECT id, name"):
		id := asInt(vals[0])
		s.f.lock.Lock()
		name, ok := s.staged[id]
		if !ok {
			name, ok = s.f.users[id]
		}
		s.f.lock.Unlock()
		if !ok {
			s.rows(s.columns(), nil, "SELECT 0", described)
		} else {
			s.rows(s.columns(), [][]any{{id, name}}, "SELECT 1", described)
		}
		return
	case sql == "SELECT COUNT(*) FROM users":
		s.f.lock.Lock()
		count := int64(len(s.f.users) + len(s.staged))
		s.f.lock.Unlock()
		s.rows(s.columns(), [][]any{{count}}, "SELECT 1", described)
		return
	case strings.HasPrefix(sql, "SELECT pg_sleep"):
		time.Sleep(time.Duration(asFloat(vals[0]) * float64(time.Second)))
		s.rows(s.columns(), [][]any{{[]byte{}}}, "SELECT 1", described)
		return
	case strings.HasPrefix(sql, "SELECT $1 AS i,"):
		b := "f"
		if vals[3].(bool) {
			b = "t"
		}
		s.rows(s.columns(), [][]any{{vals[0], vals[1], vals[2], b, append([]byte("\\x"), hex.EncodeToString(vals[4].([]byte))...)}}, "SELECT 1", described)
		return
	case strings.HasPrefix(sql, "SELECT -123::"):
		values := []any{int64(-123), int64(-123456), int64(9223372036854775807), 1.5, -2.25, "t", "f", nil, "12.345", "x", "2000-01-02", "2000-01-02 03:04:05", "2000-01-02 03:04:05+00", "00000000-0000-0000-0000-000000000001", "{\"x\":1}", "{\"x\": 1}"}
		s.rows(s.columns(), [][]any{values}, "SELECT 1", described)
		return
	case strings.HasPrefix(sql, "SELECT $1 AS value"), strings.HasPrefix(sql, "SELECT $1 AS f"):
		value := vals[0]
		if f, ok := value.(float64); ok && math.IsInf(f, 1) {
			value = "Infinity"
		}
		s.rows(s.columns(), [][]any{{value}}, "SELECT 1", described)
		return
	case sql == "SELECT 1 AS first; SELECT 2 AS second":
		s.rows([]col{{"first", 23}}, [][]any{{int64(1)}}, "SELECT 1", false)
		s.rows([]col{{"second", 23}}, [][]any{{int64(2)}}, "SELECT 1", false)
		return
	case sql == "SELECT 1":
		s.rows([]col{{"?column?", 23}}, [][]any{{int64(1)}}, "SELECT 1", described)
		return
	case sql == "SELECT 123 AS id, 'generic' AS name":
		s.rows([]col{{"id", 23}, {"name", 25}}, [][]any{{int64(123), "generic"}}, "SELECT 1", described)
		return
	}
	must(false, "unhandled SQL: "+sql)
}

func (s *session) run() {
	if !s.auth() {
		return
	}
	for {
		kind, r := s.message()
		if s.discard && kind != 'S' {
			continue
		}
		switch kind {
		case 'Q':
			s.sql, s.vals, s.oids = r.str(), nil, nil
			r.done()
			switch s.user {
			case "malformed":
				s.write(append([]byte("T"), i32(3)...))
				return
			case "truncated":
				s.send('T', append(i16(1), []byte("unterminated-name-with-no-zero")...))
				s.ready()
				continue
			case "oversized":
				s.write(append([]byte("D"), i32(0x80000000)...))
				return
			case "badrow":
				s.write(s.describe([]col{{"id", 20}}))
				s.send('D', append(i16(1), i32(0xfffffffe)...))
				s.ready()
				continue
			case "badstatus":
				s.execute(false)
				s.send('Z', []byte("x"))
				continue
			case "trickle":
				for i := 0; i < 12; i++ {
					s.send('N', []byte("SNOTICE\x00C00000\x00Mstill waiting\x00\x00"))
					time.Sleep(60 * time.Millisecond)
				}
				s.execute(false)
				s.ready()
				continue
			}
			s.execute(false)
			s.ready()
		case 'P':
			name, sql := string(r.str()), r.str()
			n := r.number(2)
			oids := make([]int, n)
			for i := range oids {
				oids[i] = r.number(4)
			}
			r.done()
			_, dup := s.statements[name]
			must(name != "" && !dup, "statement name")
			if bytes.Contains(sql, []byte("FROM nope")) || s.status == 'E' {
				state := "42P01"
				if s.status == 'E' {
					state = "25P02"
				}
				s.errorMsg(state, "prepare failed", "")
				s.discard = true
				continue
			}
			s.statements[name] = stmt{sql, oids}
			s.f.lock.Lock()
			s.f.parses++
			if len(s.statements) > s.f.maxStatements {
				s.f.maxStatements = len(s.statements)
			}
			s.f.lock.Unlock()
			must(len(s.statements) <= 256, "more than 256 statements")
			s.send('1', nil)
		case 'B':
			must(string(r.str()) == "", "portal name")
			st, ok := s.statements[string(r.str())]
			must(ok, "unknown statement")
			s.sql, s.oids = st.sql, st.oids
			nf := r.number(2)
			must(nf == 1 && r.number(2) == 1, "parameters must use binary format")
			count := r.number(2)
			must(count == len(s.oids), "parameter count")
			s.vals = nil
			for _, oid := range s.oids {
				data := r.take(r.number(4))
				var value any
				switch oid {
				case 20:
					must(len(data) == 8, "int8 length")
					value = int64(binary.BigEndian.Uint64(data))
				case 701:
					must(len(data) == 8, "float8 length")
					value = math.Float64frombits(binary.BigEndian.Uint64(data))
				case 16:
					must(len(data) == 1 && data[0] <= 1, "bool value")
					value = data[0] == 1
				default:
					must(oid == 25 || oid == 17, "parameter type")
					value = append([]byte{}, data...)
				}
				s.vals = append(s.vals, value)
			}
			must(r.number(2) == 0, "request text results")
			r.done()
			bad := false
			for i, oid := range s.oids {
				if oid == 25 && bytes.IndexByte(s.vals[i].([]byte), 0) >= 0 {
					bad = true
				}
			}
			if bad {
				s.errorMsg("22021", "invalid byte sequence for encoding UTF8", "")
				s.discard = true
			} else {
				s.send('2', nil)
			}
		case 'D':
			must(r.take(1)[0] == 'P' && len(r.str()) == 0, "Describe portal")
			r.done()
			cols := s.columns()
			if len(cols) > 0 {
				s.write(s.describe(cols))
			} else {
				s.send('n', nil)
			}
		case 'E':
			must(len(r.str()) == 0 && r.number(4) == 0, "Execute")
			r.done()
			s.execute(true)
		case 'C':
			must(r.take(1)[0] == 'S', "Close statement")
			name := string(r.str())
			r.done()
			delete(s.statements, name)
			s.f.lock.Lock()
			s.f.closes++
			s.f.lock.Unlock()
			s.send('3', nil)
		case 'S':
			r.done()
			s.discard = false
			s.ready()
		case 'X':
			return
		default:
			must(false, fmt.Sprintf("unexpected client message: %q", kind))
		}
	}
}

func (f *fake) handle(c net.Conn) {
	f.lock.Lock()
	f.conns = append(f.conns, c)
	f.open++
	if f.open > f.peak {
		f.peak = f.open
	}
	f.lock.Unlock()
	s := &session{f: f, s: c, statements: map[string]stmt{}, status: 'I', staged: map[int64][]byte{}}
	if tc, ok := c.(*net.TCPConn); ok {
		tc.SetNoDelay(true)
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				switch e := r.(type) {
				case connErr:
					// Deadlines, shutdown, bad passwords and hostile-server probes.
				case assertion:
					f.lock.Lock()
					f.errors = append(f.errors, string(e))
					f.lock.Unlock()
				default:
					f.lock.Lock()
					f.errors = append(f.errors, fmt.Sprint(r))
					f.lock.Unlock()
				}
			}
		}()
		s.run()
	}()
	// the count drops before the close, so a client that has seen the close never finds the old connection still counted
	f.lock.Lock()
	f.open--
	f.lock.Unlock()
	c.Close()
}

var (
	lock sync.Mutex
	all  = map[string]*fake{}
)

func main() {
	var cfg *tls.Config
	if len(os.Args) > 2 {
		cert, err := tls.LoadX509KeyPair(os.Args[1], os.Args[2])
		if err != nil {
			panic(err)
		}
		cfg = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}
	}
	http.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		f := &fake{users: map[int64][]byte{}, nextID: 1}
		if r.URL.Query().Get("tls") == "1" {
			f.tlsCfg = cfg
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		lock.Lock()
		all[port] = f
		lock.Unlock()
		go func() {
			for {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				go f.handle(c)
			}
		}()
		io.WriteString(w, port)
	})
	find := func(w http.ResponseWriter, r *http.Request) *fake {
		lock.Lock()
		defer lock.Unlock()
		f := all[r.URL.Query().Get("port")]
		if f == nil {
			http.Error(w, "no such server", 404)
		}
		return f
	}
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		if f := find(w, r); f != nil {
			f.lock.Lock()
			defer f.lock.Unlock()
			fmt.Fprintf(w, "auths=%d parses=%d closes=%d maxstatements=%d open=%d peak=%d errors=%d", f.auths, f.parses, f.closes, f.maxStatements, f.open, f.peak, len(f.errors))
		}
	})
	http.HandleFunc("/errors", func(w http.ResponseWriter, r *http.Request) {
		if f := find(w, r); f != nil {
			f.lock.Lock()
			defer f.lock.Unlock()
			io.WriteString(w, strings.Join(f.errors, "\n"))
		}
	})
	http.HandleFunc("/resetpeak", func(w http.ResponseWriter, r *http.Request) {
		if f := find(w, r); f != nil {
			f.lock.Lock()
			f.peak = f.open
			f.lock.Unlock()
			io.WriteString(w, "ok")
		}
	})
	http.HandleFunc("/drop", func(w http.ResponseWriter, r *http.Request) {
		if f := find(w, r); f != nil {
			f.lock.Lock()
			for _, c := range f.conns {
				c.Close()
			}
			f.conns = nil
			f.lock.Unlock()
			io.WriteString(w, "ok")
		}
	})
	panic(http.ListenAndServe("127.0.0.1:"+os.Getenv("PORT"), nil))
}
