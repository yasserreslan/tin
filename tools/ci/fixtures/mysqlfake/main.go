// The fake MySQL server of tools/ci/mysql_check.tin: just enough of MySQL for examples/mysql.tin (a users table in a map), with both
// authentication plugins (caching_sha2_password with the RSA exchange, and an auth switch to mysql_native_password) and TLS after
// SSLRequest. A control server on $PORT makes and inspects servers:
//
//	GET /new?tls=0|1       a new server on a free port (tls=1: TLS 1.3 with the certificate and key named by the arguments); answers
//	                       its port
//	GET /stats?port=P      "full=N fast=N switches=N keys=N tlsfull=N open=N peak=N"
//	GET /resetpeak?port=P  the peak of simultaneous connections starts again from the open ones
//	GET /drop?port=P       closes every connection
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	proto41 = 0x200
	secure  = 0x8000
	plugin  = 0x80000
	lenencC = 0x200000
	withDB  = 0x8
	noEOF   = 0x1000000
	sslCap  = 0x800
	caps    = 0x1 | 0x4 | withDB | proto41 | 0x2000 | secure | 0x20000 | plugin | lenencC | noEOF
)

func lenenc(n int) []byte {
	switch {
	case n < 251:
		return []byte{byte(n)}
	case n < 65536:
		return []byte{0xfc, byte(n), byte(n >> 8)}
	}
	b := make([]byte, 9)
	b[0] = 0xfe
	binary.LittleEndian.PutUint64(b[1:], uint64(n))
	return b
}

func lstr(b []byte) []byte { return append(lenenc(len(b)), b...) }

func xor(a, b []byte) []byte {
	out := make([]byte, len(a))
	for i := range a {
		out[i] = a[i] ^ b[i%len(b)]
	}
	return out
}

func sum1(b ...[]byte) []byte {
	h := sha1.New()
	for _, x := range b {
		h.Write(x)
	}
	return h.Sum(nil)
}

func sum256(b ...[]byte) []byte {
	h := sha256.New()
	for _, x := range b {
		h.Write(x)
	}
	return h.Sum(nil)
}

func coldef(name string, typ byte) []byte {
	out := lstr([]byte("def"))
	for _, s := range []string{"tin", "users", "users", name, name} {
		out = append(out, lstr([]byte(s))...)
	}
	out = append(out, 0x0c)
	var tail [10]byte
	binary.LittleEndian.PutUint16(tail[0:], 33)
	binary.LittleEndian.PutUint32(tail[2:], 255)
	tail[6] = typ
	return append(append(out, tail[:]...), 0, 0)
}

type col struct {
	name string
	typ  byte
}

var users = map[string]struct{ pw, plugin string }{
	"tin":    {"tinpass", "caching_sha2_password"},
	"native": {"nativepass", "mysql_native_password"},
}

var prepared = map[string][]col{
	"SELECT id, name FROM users WHERE id = ?": {{"id", 0x08}, {"name", 0xfd}},
	"INSERT INTO users (name) VALUES (?)":     {},
	"SELECT SLEEP(?)":                         {{"SLEEP(?)", 0x08}},
}

type fake struct {
	tlsCfg *tls.Config
	key    *rsa.PrivateKey
	pubPEM []byte

	lock      sync.Mutex
	users     map[int64][]byte
	nextID    int64
	cached    map[string]bool
	fullAuths int
	fastAuths int
	switches  int
	keyReqs   int
	tlsFull   int
	conns     []net.Conn
	open      int
	peak      int
}

type session struct {
	f     *fake
	s     net.Conn
	seq   int
	stmts map[uint32]string
	tls   bool
}

// recv reads one packet. It reads exactly the packet, never more: after an SSLRequest the TLS handshake bytes stay in the socket.
func (s *session) recv() ([]byte, error) {
	var h [4]byte
	if _, err := io.ReadFull(s.s, h[:]); err != nil {
		return nil, err
	}
	n := int(h[0]) | int(h[1])<<8 | int(h[2])<<16
	s.seq = int(h[3]) + 1
	p := make([]byte, n)
	if _, err := io.ReadFull(s.s, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *session) send(payload []byte) {
	n := len(payload)
	s.s.Write(append([]byte{byte(n), byte(n >> 8), byte(n >> 16), byte(s.seq & 255)}, payload...))
	s.seq++
}

func (s *session) ok(affected, last int) {
	s.send(append(append(append([]byte{0}, lenenc(affected)...), lenenc(last)...), 0x02, 0, 0, 0))
}

func (s *session) err(code int, state, msg string) {
	s.send(append([]byte{0xff, byte(code), byte(code >> 8), '#'}, []byte(state+msg)...))
}

func (s *session) end() { s.send([]byte{0xfe, 0, 0, 0x02, 0, 0, 0}) }

func scrambleBytes() []byte {
	b := make([]byte, 20)
	rand.Read(b)
	for i := range b {
		if b[i] == 0 {
			b[i] = 1
		}
	}
	return b
}

func (s *session) run() error {
	f := s.f
	scramble := scrambleBytes()
	c := uint32(caps)
	if f.tlsCfg != nil {
		c |= sslCap
	}
	greeting := []byte("\x0a8.0.0-fake\x00")
	greeting = append(greeting, 7, 0, 0, 0)
	greeting = append(greeting, scramble[:8]...)
	greeting = append(greeting, 0)
	var mid [8]byte
	binary.LittleEndian.PutUint16(mid[0:], uint16(c&0xffff))
	mid[2] = 255
	binary.LittleEndian.PutUint16(mid[3:], 2)
	binary.LittleEndian.PutUint16(mid[5:], uint16(c>>16))
	mid[7] = 21
	greeting = append(greeting, mid[:]...)
	greeting = append(greeting, make([]byte, 10)...)
	greeting = append(greeting, scramble[8:]...)
	greeting = append(greeting, 0)
	greeting = append(greeting, []byte("caching_sha2_password\x00")...)
	s.send(greeting)
	p, err := s.recv()
	if err != nil {
		return err
	}
	cc := binary.LittleEndian.Uint32(p[:4])
	if f.tlsCfg != nil {
		// SSLRequest (32 bytes with CLIENT_SSL), then the login packet over TLS.
		if len(p) != 32 || cc&sslCap == 0 {
			return errors.New("the client should ask for TLS")
		}
		plain := s.s
		t := tls.Server(plain, f.tlsCfg)
		if err := t.Handshake(); err != nil {
			return err
		}
		s.s = t
		s.tls = true
		if p, err = s.recv(); err != nil {
			return err
		}
		cc = binary.LittleEndian.Uint32(p[:4])
		if cc&sslCap == 0 {
			return errors.New("no CLIENT_SSL in the login")
		}
	}
	if cc&noEOF == 0 {
		return errors.New("the client should ask for DEPRECATE_EOF")
	}
	at := 32
	e := bytes.IndexByte(p[at:], 0) + at
	user := string(p[at:e])
	at = e + 1
	n := int(p[at])
	auth := p[at+1 : at+1+n]
	u, found := users[user]
	if !found {
		s.err(1045, "28000", "Access denied for user '"+user+"'")
		return nil
	}
	pw := []byte(u.pw)
	var good bool
	switch {
	case u.plugin == "mysql_native_password":
		f.lock.Lock()
		f.switches++
		f.lock.Unlock()
		scramble = scrambleBytes()
		s.send(append(append([]byte("\xfemysql_native_password\x00"), scramble...), 0))
		if auth, err = s.recv(); err != nil {
			return err
		}
		h1 := sum1(pw)
		good = bytes.Equal(auth, xor(h1, sum1(scramble, sum1(h1))))
	case func() bool { f.lock.Lock(); defer f.lock.Unlock(); return f.cached[user] }():
		h1 := sum256(pw)
		good = bytes.Equal(auth, xor(h1, sum256(sum256(h1), scramble)))
		if good {
			f.lock.Lock()
			f.fastAuths++
			f.lock.Unlock()
			s.send([]byte{1, 3})
		}
	default:
		s.send([]byte{1, 4})
		q, err := s.recv()
		if err != nil {
			return err
		}
		if s.tls {
			// Over TLS the whole password comes as it is.
			good = bytes.Equal(q, append(append([]byte{}, pw...), 0))
			if good {
				f.lock.Lock()
				f.tlsFull++
				f.cached[user] = true
				f.lock.Unlock()
			}
		} else {
			if bytes.Equal(q, []byte{2}) {
				f.lock.Lock()
				f.keyReqs++
				f.lock.Unlock()
				s.send(append([]byte{1}, f.pubPEM...))
				if q, err = s.recv(); err != nil {
					return err
				}
			}
			plain, err := rsa.DecryptOAEP(sha1.New(), nil, f.key, q, nil)
			if err != nil {
				return err
			}
			good = bytes.Equal(xor(plain, scramble), append(append([]byte{}, pw...), 0))
			if good {
				f.lock.Lock()
				f.fullAuths++
				f.cached[user] = true
				f.lock.Unlock()
			}
		}
	}
	if !good {
		s.err(1045, "28000", "Access denied for user '"+user+"'@'localhost' (using password: YES)")
		return nil
	}
	s.ok(0, 0)
	for {
		p, err := s.recv()
		if err != nil {
			return err
		}
		s.seq = 1
		switch p[0] {
		case 0x01:
			return nil
		case 0x03:
			s.query(string(p[1:]))
		case 0x16:
			s.prepare(string(p[1:]))
		case 0x17:
			s.execute(p[1:])
		case 0x19:
			delete(s.stmts, binary.LittleEndian.Uint32(p[1:5]))
		default:
			s.err(1047, "08S01", "Unknown command")
		}
	}
}

func (s *session) query(q string) {
	switch {
	case q == "DO 1" || q == "BEGIN" || q == "COMMIT" || q == "ROLLBACK" || len(q) >= 12 && q[:12] == "CREATE TABLE":
		s.ok(0, 0)
	case q == "SELECT COUNT(*) FROM users":
		s.send(lenenc(1))
		s.send(coldef("COUNT(*)", 0x08))
		s.f.lock.Lock()
		n := len(s.f.users)
		s.f.lock.Unlock()
		s.send(lstr([]byte(strconv.Itoa(n))))
		s.end()
	default:
		s.err(1064, "42000", "You have an error in your SQL syntax")
	}
}

func count(q string) int {
	n := 0
	for i := 0; i < len(q); i++ {
		if q[i] == '?' {
			n++
		}
	}
	return n
}

func (s *session) prepare(q string) {
	cols, found := prepared[q]
	if !found {
		s.err(1146, "42S02", "Table 'tin.nope' doesn't exist")
		return
	}
	id := uint32(len(s.stmts) + 1)
	s.stmts[id] = q
	params := count(q)
	var b [12]byte
	binary.LittleEndian.PutUint32(b[0:], id)
	binary.LittleEndian.PutUint16(b[4:], uint16(len(cols)))
	binary.LittleEndian.PutUint16(b[6:], uint16(params))
	s.send(append([]byte{0}, b[:]...)[:12])
	for i := 0; i < params; i++ {
		s.send(coldef("?", 0xfd))
	}
	for _, c := range cols {
		s.send(coldef(c.name, c.typ))
	}
}

func (s *session) execute(p []byte) {
	f := s.f
	id := binary.LittleEndian.Uint32(p[:4])
	q := s.stmts[id]
	n := count(q)
	at := 9 + (n+7)/8
	at++
	types := make([]byte, n)
	for i := 0; i < n; i++ {
		types[i] = p[at+2*i]
	}
	at += 2 * n
	var ints []int64
	var floats []float64
	var strs [][]byte
	for _, t := range types {
		switch t {
		case 0x08:
			ints = append(ints, int64(binary.LittleEndian.Uint64(p[at:])))
			floats = append(floats, 0)
			strs = append(strs, nil)
			at += 8
		case 0x05:
			fl := math.Float64frombits(binary.LittleEndian.Uint64(p[at:]))
			ints = append(ints, int64(fl))
			floats = append(floats, fl)
			strs = append(strs, nil)
			at += 8
		case 0x01:
			ints = append(ints, int64(p[at]))
			floats = append(floats, float64(p[at]))
			strs = append(strs, nil)
			at++
		default:
			ln := int(p[at])
			at++
			if ln == 0xfc {
				ln = int(binary.LittleEndian.Uint16(p[at:]))
				at += 2
			}
			ints = append(ints, 0)
			floats = append(floats, 0)
			strs = append(strs, p[at:at+ln])
			at += ln
		}
	}
	switch {
	case len(q) >= 6 && q[:6] == "INSERT":
		f.lock.Lock()
		f.nextID++
		uid := f.nextID
		f.users[uid] = append([]byte{}, strs[0]...)
		f.lock.Unlock()
		s.ok(1, int(uid))
	case len(q) >= 12 && q[:12] == "SELECT SLEEP":
		secs := floats[0]
		if types[0] == 0x08 {
			secs = float64(ints[0])
		}
		time.Sleep(time.Duration(secs * float64(time.Second)))
		s.send(lenenc(1))
		s.send(coldef("SLEEP(?)", 0x08))
		s.send(append([]byte{0, 0}, make([]byte, 8)...))
		s.end()
	default:
		s.send(lenenc(2))
		s.send(coldef("id", 0x08))
		s.send(coldef("name", 0xfd))
		f.lock.Lock()
		name, found := f.users[ints[0]]
		f.lock.Unlock()
		if found {
			row := append([]byte{0, 0}, make([]byte, 8)...)
			binary.LittleEndian.PutUint64(row[2:], uint64(ints[0]))
			s.send(append(row, lstr(name)...))
		}
		s.end()
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
	s := &session{f: f, s: c, stmts: map[uint32]string{}}
	s.run()
	// the count drops before the close, so a client that has seen the close never finds the old connection still counted
	f.lock.Lock()
	f.open--
	f.lock.Unlock()
	c.Close()
	s.s.Close()
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
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	http.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		f := &fake{users: map[int64][]byte{}, cached: map[string]bool{}, key: key, pubPEM: pubPEM}
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
			fmt.Fprintf(w, "full=%d fast=%d switches=%d keys=%d tlsfull=%d open=%d peak=%d", f.fullAuths, f.fastAuths, f.switches, f.keyReqs, f.tlsFull, f.open, f.peak)
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
