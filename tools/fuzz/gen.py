#!/usr/bin/env python3
"""Seeded, bounded shared AST, rendered as edition-1 Tin and Go 1.22+."""
import argparse
from dataclasses import asdict, dataclass, field
import json
from pathlib import Path
import random

TYPES = {'i8': 'int8', 'i16': 'int16', 'i32': 'int32', 'i64': 'int64',
         'u8': 'uint8', 'u16': 'uint16', 'u32': 'uint32', 'u64': 'uint64',
         'f32': 'float32', 'f64': 'float64', 'str': 'string', 'bool': 'bool'}


@dataclass
class Expr:
    typ: str
    op: str
    value: str = ''
    args: list = field(default_factory=list)

    def render(self, lang):
        values = [a.render(lang) for a in self.args]
        if self.op == 'atom':
            return self.value
        if self.op == 'convert':
            return f'{self.typ if lang == "tin" else TYPES[self.typ]}({values[0]})'
        if self.op == 'not':
            return f'({"~" if lang == "tin" else "^"}{values[0]})'
        op = self.op if lang == 'tin' else self.op.replace('%', '') if self.op in ('+%', '-%', '*%') else self.op
        return f'({values[0]} {op} {values[1]})'


@dataclass
class Node:
    """A declaration, statement, or block; paired syntax shares children and expressions."""
    tin: str
    go: str
    children: list = field(default_factory=list)
    end_tin: str = ''
    end_go: str = ''
    exprs: list = field(default_factory=list)
    kind: str = 'statement'

    def render(self, lang, depth=0):
        template = self.tin if lang == 'tin' else self.go
        for i, expr in enumerate(self.exprs):
            template = template.replace(f'${i}', expr.render(lang))
        lines = [('\t' * depth + line) for line in template.splitlines()]
        for child in self.children:
            lines.append(child.render(lang, depth + 1))
        end = self.end_tin if lang == 'tin' else self.end_go
        lines.extend('\t' * depth + line for line in end.splitlines())
        return '\n'.join(lines)


@dataclass
class Program:
    seed: int
    size: int
    declarations: list
    blocks: list

    def render(self, lang):
        imports = ('import "say"\nimport "constraints"\nimport "sift"' if lang == 'tin'
                   else 'import "fmt"')
        decls = '\n\n'.join(n.render(lang) for n in self.declarations)
        body = '\n'.join(n.render(lang, 1) for n in self.blocks)
        return (f'// fuzz seed={self.seed} size={self.size}\npackage main\n\n{imports}\n\n'
                f'{decls}\n\n{"fn" if lang == "tin" else "func"} main() {{\n{body}\n}}\n')

    def save(self, path):
        Path(path).write_text(json.dumps(asdict(self), indent=2) + '\n')

    @classmethod
    def load(cls, path):
        def expr(d):
            return Expr(d['typ'], d['op'], d['value'], [expr(a) for a in d['args']])
        def node(d):
            return Node(d['tin'], d['go'], [node(c) for c in d['children']],
                        d['end_tin'], d['end_go'], [expr(e) for e in d['exprs']], d['kind'])
        d = json.loads(Path(path).read_text())
        return cls(d['seed'], d['size'], [node(n) for n in d['declarations']],
                   [node(n) for n in d['blocks']])


def atom(typ, value):
    return Expr(typ, 'atom', str(value))


def output(tin, go=None, exprs=None):
    return Node('say.Line(' + tin + ')', 'fmt.Println(' + (go or tin) + ')', exprs=exprs or [])


def integer_expr(rng, typ, depth):
    if depth == 0 or rng.randrange(4) == 0:
        return atom(typ, rng.choice(['a', 'b']))
    op = rng.choice(['+%', '-%', '*%', '&', '|', '^', '<<', '>>', '/', '%', 'not'])
    left = integer_expr(rng, typ, depth - 1)
    width = int(typ[1:])
    if op == 'not':
        return Expr(typ, op, args=[left])
    if op in ('<<', '>>'):
        right = Expr(typ, '&', args=[atom(typ, 'b'), atom(typ, width - 1)])
    elif op in ('/', '%'):
        # Strictly positive, bounded divisor: excludes zero and signed MIN / -1.
        right = Expr(typ, '|', args=[Expr(typ, '&', args=[atom(typ, 'b'), atom(typ, 63)]), atom(typ, 1)])
    else:
        right = integer_expr(rng, typ, depth - 1)
    return Expr(typ, op, args=[left, right])


def declarations():
    return [
        Node('type Point struct {\nx i64\ny i64\n}', 'type Point struct {\nx int64\ny int64\n}', kind='declaration'),
        Node('fn (p Point) sum() i64 {\nreturn p.x +% p.y\n}', 'func (p *Point) sum() int64 {\nreturn p.x + p.y\n}', kind='declaration'),
        Node('fn (p mut Point) move(d i64) {\np.x +%= d\n}', 'func (p *Point) move(d int64) {\np.x += d\n}', kind='declaration'),
        Node('@wrap fn wrapAdd(a i64, b i64) i64 {\nreturn a + b\n}', 'func wrapAdd(a int64, b int64) int64 {\nreturn a + b\n}', kind='declaration'),
        Node('fn recur(n i64) i64 {\nif n == 0 {\nreturn 1\n}\nreturn n + recur(n - 1)\n}', 'func recur(n int64) int64 {\nif n == 0 {\nreturn 1\n}\nreturn n + recur(n - 1)\n}', kind='declaration'),
        Node('fn many(' + ', '.join(f'p{i} i64' for i in range(12)) + ') (i64, i64) {\nreturn p0 +% p11, p1 ^ p10\n}', 'func many(' + ', '.join(f'p{i} int64' for i in range(12)) + ') (int64, int64) {\nreturn p0 + p11, p1 ^ p10\n}', kind='declaration'),
        Node('fn identity[T constraints.Any](x T) T {\nreturn x\n}', 'func identity[T any](x T) T {\nreturn x\n}', kind='declaration'),
        Node('fn larger[T i64 | i32](a T, b T) T {\nif a > b {\nreturn a\n}\nreturn b\n}', 'func larger[T int64 | int32](a T, b T) T {\nif a > b {\nreturn a\n}\nreturn b\n}', kind='declaration'),
        Node('fn ordered[T sift.Ordered](x T) T {\nreturn x\n}', 'func ordered[T ~int64 | ~string](x T) T {\nreturn x\n}', kind='declaration'),
        Node('type Box[T constraints.Any] struct {\nv T\n}', 'type Box[T any] struct {\nv T\n}', kind='declaration'),
        Node('fn checked(x i64) !i64 {\nif x < 0 {\nfail "negative"\n}\nreturn x\n}', 'func checked(x int64) (int64, error) {\nif x < 0 {\nreturn 0, fmt.Errorf("negative")\n}\nreturn x, nil\n}', kind='declaration'),
        Node('fn forwarded(x i64) !i64 {\nlet v = try checked(x)\nreturn v\n}', 'func forwarded(x int64) (int64, error) {\nv, err := checked(x)\nif err != nil {\nreturn 0, err\n}\nreturn v, nil\n}', kind='declaration'),
        Node('type Choice enum {\nValue(i64)\nEmpty\n}', 'type Choice struct {\ntag int\nv int64\n}', kind='declaration')]


def generate(seed, size=12):
    if not 1 <= size <= 1000:
        raise ValueError('size must be between 1 and 1000')
    rng = random.Random(seed)
    blocks = []
    for index in range(size):
        typ = list(TYPES)[:8][index % 8]
        width = int(typ[1:])
        lo = -(1 << (width - 1)) if typ[0] == 'i' else 0
        hi = (1 << (width - (typ[0] == 'i'))) - 1
        a, b = (rng.choice([lo, hi, 0, 1, rng.randint(lo, hi)]) for _ in range(2))
        statements = [Node(f'let a {typ} = {a}', f'var a {TYPES[typ]} = {a}'),
                      Node(f'let b {typ} = {b}', f'var b {TYPES[typ]} = {b}'),
                      output(f'"int-{index}", $0', exprs=[integer_expr(rng, typ, rng.randint(1, 4))]),
                      output('a == b, a != b, a < b, a <= b, a > b, a >= b'),
                      output(', '.join(f'{t}(a)' for t in list(TYPES)[:8]),
                             ', '.join(f'{TYPES[t]}(a)' for t in list(TYPES)[:8]))]
        blocks.append(Node('{', '{', statements, '}', '}'))
    # Small inputs prove ordinary arithmetic, negation, floats and float->int stay in range.
    x, y = rng.randint(1, 20), rng.randint(1, 20)
    statements = [Node(f'let x i64 = {x}\nlet y i64 = {y}', f'var x int64 = {x}\nvar y int64 = {y}'),
                  output('"safe", x + y, x - y, x * y, -x, !(x == y), x < y && y > 0 || x == y'),
                  Node('mut total i64 = 0', 'var total int64 = 0'),
                  Node('if x < y {', 'if x < y {', [Node('total = x', 'total = x')],
                       '} else {\ntotal = y\n}', '} else {\ntotal = y\n}'),
                  Node('for i in 0..8 {', 'for i := int64(0); i < 8; i++ {',
                       [Node('if i == 2 {\ncontinue\n}\nif i == 6 {\nbreak\n}\ntotal += i',
                             'if i == 2 {\ncontinue\n}\nif i == 6 {\nbreak\n}\ntotal += i')], '}', '}'),
                  Node('mut n i64 = 0', 'var n int64 = 0'),
                  Node('for n < 3 {', 'for n < 3 {', [Node('n += 1\ntotal += n', 'n += 1\ntotal += n')], '}', '}'),
                  output('"flow", total, recur(x % 6), wrapAdd(x, y)'),
                  Node('let (r, s) = many(' + ', '.join(['x', 'y'] * 6) + ')',
                       'r, s := many(' + ', '.join(['x', 'y'] * 6) + ')'), output('r, s')]
    # Separate scopes permit f32 and f64 to use the same identifiers.
    for t in ('f32', 'f64'):
        conversions = list(TYPES)[:10]
        blocks.append(Node('{', '{', [Node(f'let f {t} = {t}({x}) / {t}(4)\nlet g {t} = {y}',
                                                       f'var f {TYPES[t]} = {TYPES[t]}({x}) / {TYPES[t]}(4)\nvar g {TYPES[t]} = {y}'),
                                         output('"float", f + g, f - g, f * g, f / g, f < g'),
                                         output(', '.join(f'{v}(f)' for v in conversions), ', '.join(f'{TYPES[v]}(f)' for v in conversions))], '}', '}'))
    statements += [Node('let text = "héllo" + "Tin"', 'text := "héllo" + "Tin"'),
                   output('"str", len(text), text[1:4], text[0], text < "z", text == "x"', '"str", int64(len(text)), text[1:4], text[0], text < "z", text == "x"'),
                   Node('let bytes = []u8{65, 66, 67}', 'bytes := []uint8{65, 66, 67}'),
                   output('str(bytes), "value={x}"', 'string(bytes), fmt.Sprint("value=", x)'),
                   Node('mut xs = []i64{x, y}', 'xs := []int64{x, y}'),
                   Node('xs = append(xs, total)\nxs[0] = total', 'xs = append(xs, total)\nxs[0] = total'),
                   Node('let zs = make([]i64, 3)\nzs[1] = x', 'zs := make([]int64, 3)\nzs[1] = x'),
                   Node('let array = [3]i64{1, 2}\nlet alias = array\nalias[2] = x', 'array := []int64{1, 2, 0}\nalias := array\nalias[2] = x'),
                   Node('for i, v in xs {', 'for i, v := range xs {', [output('"slice", i, v', '"slice", int64(i), v')], '}', '}'),
                   output('xs, zs, array, len(xs)', 'xs, zs, array, int64(len(xs))'),
                   Node('let m = map[str]i64{"b": x, "a": y}\nm["c"] = total\nlet (v, ok) = m["missing"]\ndelete(m, "b")', 'm := map[string]int64{"b": x, "a": y}\nm["c"] = total\nv, ok := m["missing"]\ndelete(m, "b")'),
                   output('"map", m, v, ok, len(m)', '"map", m, v, ok, int64(len(m))'),
                   Node('let p = Point{x: x, y: y}\nlet q = p\nq.move(3)\nlet other = Point{x: x, y: y}', 'p := &Point{x: x, y: y}\nq := p\nq.move(3)\nother := &Point{x: x, y: y}'),
                   output('"struct", p.x, p.sum(), p == other', '"struct", p.x, p.sum(), *p == *other'),
                   Node('match x % 3 {\n0 => total = 10\n1 => total = 20\n_ => total = 30\n}', 'switch x % 3 {\ncase 0: total = 10\ncase 1: total = 20\ndefault: total = 30\n}'), output('"match", total'),
                   Node('let f = fn(a i64) i64 {\nreturn a +% total\n}\nlet fv = wrapAdd', 'f := func(a int64) int64 {\nreturn a + total\n}\nfv := wrapAdd'),
                   output('"closure", f(x), fv(x, y)'),
                   Node('let box = Box[i64]{v: x}', 'box := &Box[int64]{v: x}'),
                   output('"generic", identity[i64](box.v), larger[i32](i32(x), i32(y)), ordered[str](text)', '"generic", identity[int64](box.v), larger[int32](int32(x), int32(y)), ordered[string](text)'),
                   Node('let opt ?i64 = x', 'opt := &x'),
                   Node('if opt != nil {', 'if opt != nil {', [output('"optional", opt', '"optional", *opt')], '}', '}'),
                   Node('let good = forwarded(x) catch _ { -7 }', 'good, err := forwarded(x)\nif err != nil {\ngood = -7\n}'),
                   Node('let bad = forwarded(-x) catch _ { -7 }', 'bad, err := forwarded(-x)\nif err != nil {\nbad = -7\n}'), output('"fault", good, bad'),
                   Node('let choice = Choice.Value(x)\nmatch choice {\nValue(v) => total = v\nEmpty => total = 0\n}', 'choice := Choice{tag: 1, v: x}\nswitch choice.tag {\ncase 1: total = choice.v\ndefault: total = 0\n}'), output('"enum", total')]
    blocks.append(Node('{', '{', statements, '}', '}'))
    return Program(seed, size, declarations(), blocks)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--seed', type=int, required=True)
    parser.add_argument('--size', type=int, default=12)
    parser.add_argument('--tin', type=Path, required=True)
    parser.add_argument('--go', type=Path, required=True)
    parser.add_argument('--ast', type=Path)
    args = parser.parse_args()
    program = generate(args.seed, args.size)
    args.tin.write_text(program.render('tin'))
    args.go.write_text(program.render('go'))
    if args.ast:
        program.save(args.ast)


if __name__ == '__main__':
    main()
