// Command list_ring reads operation scripts on stdin, one case per line, and runs each on Go's container/list
// and container/ring. tools/ci/fixtures/list_ring.tin runs the same scripts on cairn's List and Ring, and
// tools/ci/list_ring_check.tin compares the two outputs line by line (#919).
//
// A case is operations separated by " ; ". An operation is a name and its operands:
//
//	pf L V, pb L V          PushFront, PushBack of the integer V on list L (a or b)
//	ib L E V, ia L E V      InsertBefore, InsertAfter of V on L relative to element E (nil when E is not in L)
//	pfl L M, pbl L M        PushFrontList, PushBackList: a copy of M's values into L (M may be L)
//	rm L E                  Remove E from L; prints the value it returns
//	mtf L E, mtb L E        MoveToFront, MoveToBack of E on L
//	fr L, bk L              Front, Back of L
//	nx X, pv X              Next, Prev of an element (eK) or a ring node (rK)
//	new V...                ring.New with one node per value, the values in order (nil with none)
//	link R S                Link: R's next becomes S; prints the old R.Next()
//	unlink R N, move R N    Unlink N nodes after R (nil when N <= 0), Move N steps (back when negative)
//	do R                    Do: the values of R's ring, in order
//	len R                   Len of R's ring
//
// Handles eK and rK name the elements and ring nodes the case has seen, numbered in the order it first
// met them; a handle that has not been seen yet prints "? op" and does nothing. After each operation every
// list (forward and backward values) and every handle (value, owning list, next and prev) is printed.
package main

import (
	"bufio"
	"container/list"
	"container/ring"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type state struct {
	lists [2]*list.List
	elems []*list.Element
	nodes []*ring.Ring
}

// elemID returns e's handle number, numbering it when it is new.
func (s *state) elemID(e *list.Element) int {
	for i, x := range s.elems {
		if x == e {
			return i
		}
	}
	s.elems = append(s.elems, e)
	return len(s.elems) - 1
}

// nodeID returns r's handle number, numbering it when it is new.
func (s *state) nodeID(r *ring.Ring) int {
	for i, x := range s.nodes {
		if x == r {
			return i
		}
	}
	s.nodes = append(s.nodes, r)
	return len(s.nodes) - 1
}

// elemRef prints an element as its handle, or nil.
func (s *state) elemRef(e *list.Element) string {
	if e == nil {
		return "nil"
	}
	return fmt.Sprintf("e%d", s.elemID(e))
}

// nodeRef prints a ring node as its handle, or nil.
func (s *state) nodeRef(r *ring.Ring) string {
	if r == nil {
		return "nil"
	}
	return fmt.Sprintf("r%d", s.nodeID(r))
}

// elem resolves an element handle token; ok is false when the handle is not one the case has seen.
func (s *state) elem(tok string) (*list.Element, bool) {
	k, ok := handle(tok, 'e', len(s.elems))
	if !ok {
		return nil, false
	}
	return s.elems[k], true
}

// node resolves a ring node handle token.
func (s *state) node(tok string) (*ring.Ring, bool) {
	k, ok := handle(tok, 'r', len(s.nodes))
	if !ok {
		return nil, false
	}
	return s.nodes[k], true
}

// handle parses a handle token of the given prefix that names one of the n handles seen so far.
func handle(tok string, prefix byte, n int) (int, bool) {
	if len(tok) < 2 || tok[0] != prefix {
		return 0, false
	}
	k, err := strconv.Atoi(tok[1:])
	if err != nil || k < 0 || k >= n {
		return 0, false
	}
	return k, true
}

// list resolves a list name.
func (s *state) list(tok string) *list.List {
	if tok == "a" {
		return s.lists[0]
	}
	return s.lists[1]
}

// num parses an integer operand.
func num(tok string) int {
	v, _ := strconv.Atoi(tok)
	return v
}

// exec runs one operation and returns its result text, or "?" for an unknown handle.
func (s *state) exec(f []string) string {
	unknown := "?"
	switch f[0] {
	case "pf":
		return s.elemRef(s.list(f[1]).PushFront(num(f[2])))
	case "pb":
		return s.elemRef(s.list(f[1]).PushBack(num(f[2])))
	case "ib", "ia":
		mark, ok := s.elem(f[2])
		if !ok {
			return unknown
		}
		l := s.list(f[1])
		var e *list.Element
		if f[0] == "ib" {
			e = l.InsertBefore(num(f[3]), mark)
		} else {
			e = l.InsertAfter(num(f[3]), mark)
		}
		return s.elemRef(e)
	case "pfl":
		s.list(f[1]).PushFrontList(s.list(f[2]))
		return "-"
	case "pbl":
		s.list(f[1]).PushBackList(s.list(f[2]))
		return "-"
	case "rm":
		e, ok := s.elem(f[2])
		if !ok {
			return unknown
		}
		return fmt.Sprint(s.list(f[1]).Remove(e))
	case "mtf", "mtb":
		e, ok := s.elem(f[2])
		if !ok {
			return unknown
		}
		if f[0] == "mtf" {
			s.list(f[1]).MoveToFront(e)
		} else {
			s.list(f[1]).MoveToBack(e)
		}
		return "-"
	case "fr":
		return s.elemRef(s.list(f[1]).Front())
	case "bk":
		return s.elemRef(s.list(f[1]).Back())
	case "nx", "pv":
		if e, ok := s.elem(f[1]); ok {
			if f[0] == "nx" {
				return s.elemRef(e.Next())
			}
			return s.elemRef(e.Prev())
		}
		r, ok := s.node(f[1])
		if !ok {
			return unknown
		}
		if f[0] == "nx" {
			return s.nodeRef(r.Next())
		}
		return s.nodeRef(r.Prev())
	case "new":
		if len(f) == 1 {
			return "nil"
		}
		r := ring.New(len(f) - 1)
		first := len(s.nodes)
		for i, tok := range f[1:] {
			r.Value = num(tok)
			s.nodeID(r)
			r = r.Next()
			_ = i
		}
		return fmt.Sprintf("r%d", first)
	case "link":
		r, ok1 := s.node(f[1])
		t, ok2 := s.node(f[2])
		if !ok1 || !ok2 {
			return unknown
		}
		return s.nodeRef(r.Link(t))
	case "unlink":
		r, ok := s.node(f[1])
		if !ok {
			return unknown
		}
		return s.nodeRef(r.Unlink(num(f[2])))
	case "move":
		r, ok := s.node(f[1])
		if !ok {
			return unknown
		}
		return s.nodeRef(r.Move(num(f[2])))
	case "do":
		r, ok := s.node(f[1])
		if !ok {
			return unknown
		}
		var vals []string
		r.Do(func(v any) { vals = append(vals, fmt.Sprint(v)) })
		return "[" + strings.Join(vals, " ") + "]"
	case "len":
		r, ok := s.node(f[1])
		if !ok {
			return unknown
		}
		return strconv.Itoa(r.Len())
	}
	return unknown
}

// dump prints both lists and every handle. Every live element is numbered first, in list order (a, then b),
// so the numbering does not depend on what the operations happened to name.
func (s *state) dump(w *bufio.Writer) {
	var live []*list.Element
	for _, l := range s.lists {
		for e := l.Front(); e != nil; e = e.Next() {
			live = append(live, e)
		}
	}
	for _, e := range live {
		s.elemID(e)
	}
	owner := func(e *list.Element) string {
		for i, l := range s.lists {
			for x := l.Front(); x != nil; x = x.Next() {
				if x == e {
					return string(rune('a' + i))
				}
			}
		}
		return "-"
	}
	for i, name := range []string{"a", "b"} {
		l := s.lists[i]
		var fwd, rev []string
		for e := l.Front(); e != nil; e = e.Next() {
			fwd = append(fwd, fmt.Sprint(e.Value))
		}
		for e := l.Back(); e != nil; e = e.Prev() {
			rev = append(rev, fmt.Sprint(e.Value))
		}
		fmt.Fprintf(w, "list %s len=%d front=%s back=%s fwd=[%s] rev=[%s]\n", name, l.Len(),
			s.elemRef(l.Front()), s.elemRef(l.Back()), strings.Join(fwd, " "), strings.Join(rev, " "))
	}
	for id := 0; id < len(s.elems); id++ {
		e := s.elems[id]
		fmt.Fprintf(w, "e%d val=%v own=%s next=%s prev=%s\n", id, e.Value, owner(e), s.elemRef(e.Next()), s.elemRef(e.Prev()))
	}
	for id := 0; id < len(s.nodes); id++ {
		r := s.nodes[id]
		fmt.Fprintf(w, "r%d val=%v next=%s prev=%s\n", id, r.Value, s.nodeRef(r.Next()), s.nodeRef(r.Prev()))
	}
}

// runCase runs the operations of one case line and prints the results.
func runCase(line string, w *bufio.Writer) {
	s := &state{}
	s.lists[0] = list.New()
	s.lists[1] = list.New()
	for _, op := range strings.Split(line, " ; ") {
		f := strings.Fields(op)
		if len(f) == 0 {
			continue
		}
		fmt.Fprintf(w, "%s => %s\n", strings.Join(f, " "), s.exec(f))
		s.dump(w)
	}
}

func main() {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	n := 0
	for in.Scan() {
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		n++
		fmt.Fprintf(w, "case %d\n", n)
		runCase(line, w)
	}
}
