#!/usr/bin/env python3
"""Token-level front-end mutator for Tin compiler inputs."""
from __future__ import annotations

import argparse
import concurrent.futures
import glob
import tempfile
import os
import random
import re
import signal
import subprocess
import sys
import time
from dataclasses import dataclass
from pathlib import Path

TOKEN = re.compile(r'''(?P<space>\s+)|(?P<comment>//[^\n]*)|(?P<raw>`(?:``|[^`])*`)|(?P<string>"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')|(?P<number>(?:0[xX][\da-fA-F_]+|0[bB][01_]+|0[oO][0-7_]+|\d[\d_]*(?:\.\d[\d_]*)?(?:[eE][+-]?\d[\d_]*)?)(?:ns|us|µs|ms|s|m|h|kb|mb|gb)?)|(?P<ident>[\w]+)|(?P<op>\+%|-%|\*%|\.\.|=>|->|==|!=|<=|>=|&&|\|\||<<|>>|[+*/%&|^~!=<>:.,;(){}\[\]-])''', re.UNICODE | re.X)
KEYWORDS = 'package import fn let mut const type struct value enum shape use on if else for in break continue return fail try catch match true false nil'.split()
INSERT = KEYWORDS + list('(){}[]') + ['+', '-', '*', '/', '%', '==', '!=', '<', '>', '&&', '||', '=>', '->', '+%', '-%', '*%', '..']

@dataclass(frozen=True)
class Token:
    text: str
    start: int
    end: int


def tokenize(text: str, *, trivia: bool = False) -> list[Token]:
    """Lex tokens while treating complete quoted forms (including interpolation) atomically."""
    out: list[Token] = []
    i = 0
    while i < len(text):
        start = i
        c = text[i]
        if c.isspace():
            i += 1
            while i < len(text) and text[i].isspace(): i += 1
            kind = 'space'
        elif text.startswith('//', i):
            i = text.find('\n', i)
            if i < 0: i = len(text)
            kind = 'comment'
        elif c == '`':
            i += 1
            while i < len(text) and text[i] != '`': i += 1
            if i < len(text): i += 1
            kind = 'raw'
        elif c in '\"\'':
            quote = c
            i += 1
            depth = 0
            while i < len(text):
                ch = text[i]
                if ch == '\\':
                    i += 2; continue
                if quote == '\"' and ch == '{': depth += 1
                elif quote == '\"' and ch == '}' and depth: depth -= 1
                elif ch == quote and depth == 0:
                    i += 1; break
                i += 1
            kind = 'string' if quote == '\"' else 'rune'
        elif c.isdigit():
            m = TOKEN.match(text, i)
            if m and m.lastgroup == 'number': i = m.end()
            else: i += 1
            kind = 'number'
        elif c == '_' or c.isalpha():
            i += 1
            while i < len(text) and (text[i] == '_' or text[i].isalnum()): i += 1
            kind = 'ident'
        else:
            op = next((x for x in ('+%', '-%', '*%', '..', '=>', '->', '==', '!=', '<=', '>=', '&&', '||', '<<', '>>') if text.startswith(x, i)), None)
            i += len(op) if op else 1
            kind = 'op'
        if trivia or kind not in ('space', 'comment'):
            out.append(Token(text[start:i], start, i))
    return out


def round_trip(text: str) -> str:
    return ''.join(token.text for token in tokenize(text, trivia=True))


def emit_mutation(src: str, rng: random.Random, other: str | None = None) -> tuple[str, str]:
    toks = tokenize(src)
    if not toks:
        return src, 'empty'
    choices = ['delete', 'duplicate', 'swap', 'move', 'insert', 'splice', 'truncate']
    op = rng.choice(choices if other is not None else choices[:-2] + ['truncate'])
    vals = [t.text for t in toks]
    i = rng.randrange(len(vals))
    if op == 'delete':
        del vals[i]
    elif op == 'duplicate':
        vals.insert(i, vals[i])
    elif op == 'swap' and len(vals) > 1:
        j = rng.randrange(len(vals)); vals[i], vals[j] = vals[j], vals[i]
    elif op == 'move' and len(vals) > 1:
        v = vals.pop(i); vals.insert(rng.randrange(len(vals) + 1), v)
    elif op == 'insert':
        vals.insert(i, rng.choice(INSERT))
    elif op == 'splice' and other:
        b = [t.text for t in tokenize(other)]
        if b:
            j = rng.randrange(len(b)); k = rng.randrange(len(b) + 1)
            vals[i:i] = b[j:k]
    elif op == 'truncate':
        del vals[rng.randrange(len(vals) + 1):]
    # Whitespace between lexical tokens keeps identifiers and operators separable.
    return ' '.join(vals) + '\n', op


def candidates(root: Path, explicit: list[str]) -> list[Path]:
    found = [Path(x) for x in explicit]
    for pattern in ('toolchain/tests/v2/**/*.tin', 'toolchain/tests/edition1/**/*.tin', 'examples/*.tin'):
        found.extend(Path(p) for p in glob.glob(str(root / pattern), recursive=True))
    return sorted({p.resolve() for p in found if p.is_file()})


def check_one(args: tuple[int, str, str, str, int, float]) -> tuple[int, str | None]:
    idx, compiler, mutated, outdir, seed, timeout, flags, origin, kind = args
    target = Path(outdir) / f'mutant-{idx:08d}.tin'
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(mutated, encoding='utf-8', errors='surrogateescape')
    try:
        with tempfile.NamedTemporaryFile(mode='w', encoding='utf-8', errors='surrogateescape', suffix='.tin', dir=Path(origin).parent, delete=True) as scratch:
            scratch.write(mutated); scratch.flush()
            p = subprocess.run([compiler, *flags, scratch.name], cwd=Path(origin).parent, capture_output=True, text=True, errors='surrogateescape', timeout=timeout, env={**os.environ, 'TIN_ROOT': str(Path.cwd())})
    except subprocess.TimeoutExpired:
        return idx, f'hang after {timeout}s ({kind})'
    except OSError as e:
        return idx, f'compiler launch failed: {e}'
    diag = (p.stdout + '\n' + p.stderr).strip()
    if p.returncode < 0:
        return idx, f'signal {-p.returncode} ({kind})'
    if p.returncode == 2:
        return idx, f'exit 2 ({kind}): {diag[:500]}'
    if 'E990 INTERNAL' in diag:
        return idx, f'E990 INTERNAL ({kind}): {diag[:500]}'
    if p.returncode == 1:
        if not re.search(r'[^\s:]+:\d+:\d+.*\bE\d{3}\b', diag):
            return idx, f'diagnostic lacks file:line:col and E code ({kind}): {diag[:500]}'
    elif p.returncode != 0:
        return idx, f'unexpected exit {p.returncode} ({kind}): {diag[:500]}'
    return idx, None


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--compiler', default=os.environ.get('TINC', 'bin/tinc'))
    ap.add_argument('--root', default='.')
    ap.add_argument('--input', action='append', default=[], help='additional source file; repeatable')
    ap.add_argument('--seconds', type=float, default=60)
    ap.add_argument('--count', type=int, default=None)
    ap.add_argument('--jobs', type=int, default=1)
    ap.add_argument('--out', default='bin/fuzz-fe')
    ap.add_argument('--seed', type=int, default=1)
    ap.add_argument('--timeout', type=float, default=10)
    a = ap.parse_args()
    root = Path(a.root).resolve()
    compiler = str(Path(a.compiler).resolve())
    files = candidates(root, a.input)
    if not files:
        ap.error('no corpus inputs found')
    # Establish and persist a baseline before mutating. Inputs with existing compiler
    # failures are recorded and excluded so they are never attributed to a mutation.
    out = Path(a.out); out.mkdir(parents=True, exist_ok=True)
    sources: list[tuple[Path, str]] = []
    baseline_rows = []
    for path in files:
        src = path.read_text(encoding='utf-8', errors='surrogateescape')
        try:
            flags = ['-edition', '1', '-parse-only'] if '/toolchain/tests/edition1/' in str(path) else ['-S']
            p = subprocess.run([compiler, *flags, str(path)], cwd=root, capture_output=True, text=True, errors='surrogateescape', timeout=a.timeout, env={**os.environ, 'TIN_ROOT': str(root)})
        except subprocess.TimeoutExpired:
            baseline_rows.append(f'unclean\ttimeout\t{path}')
            continue
        except OSError as e:
            print(f'compiler launch failed: {e}', file=sys.stderr); return 2
        diag = p.stdout + '\n' + p.stderr
        clean = p.returncode == 0 or (p.returncode == 1 and re.search(r'[^\s:]+:\d+:\d+[^\n]*\bE\d{3}\b', diag))
        if p.returncode < 0:
            reason = f'signal:{-p.returncode}'
        elif 'E990 INTERNAL' in diag:
            reason = 'E990_INTERNAL'
        elif p.returncode == 2:
            reason = 'exit2'
        elif p.returncode == 1 and not clean:
            reason = 'diagnostic_without_file_line_col_Ecode'
        elif p.returncode not in (0, 1):
            reason = f'exit:{p.returncode}'
        else:
            reason = 'clean'
        baseline_rows.append(f'{"clean" if clean else "unclean"}\t{reason}\t{path}')
        if clean:
            sources.append((path, src))
        else:
            print(f'baseline excluded ({reason}): {path}', file=sys.stderr)
    (out / 'baseline.tsv').write_text('\n'.join(baseline_rows) + '\n')
    print(f'baseline: {len(sources)} clean, {len(files)-len(sources)} excluded; see {out / "baseline.tsv"}', flush=True)
    if not sources:
        print('no clean baseline inputs to mutate', file=sys.stderr); return 2
    rng = random.Random(a.seed)
    start, idx = time.monotonic(), 0
    limit = a.count if a.count is not None else sys.maxsize
    failures = 0
    with concurrent.futures.ThreadPoolExecutor(max_workers=max(1, a.jobs)) as pool:
        pending = set()
        while idx < limit and (a.count is not None or time.monotonic() - start < a.seconds):
            while len(pending) < max(1, a.jobs) and idx < limit and (a.count is not None or time.monotonic() - start < a.seconds):
                path, src = sources[rng.randrange(len(sources))]
                partner = sources[rng.randrange(len(sources))][1]
                mutated, kind = emit_mutation(src, random.Random(a.seed + idx), partner)
                dst = out / f'mutant-{idx:08d}.tin'; dst.write_text(mutated, encoding='utf-8', errors='surrogateescape')
                # Run the already materialized source through the same classification logic.
                flags = ['-edition', '1', '-parse-only'] if '/toolchain/tests/edition1/' in str(path) else ['-S']
                f = pool.submit(check_one, (idx, compiler, mutated, str(out), a.seed, a.timeout, flags, str(path), kind))
                pending.add(f); idx += 1
            done, pending = concurrent.futures.wait(pending, return_when=concurrent.futures.FIRST_COMPLETED)
            for f in done:
                n, error = f.result()
                if error:
                    failures += 1
                    print(f'FUZZER ERROR mutant {n}: {error}', file=sys.stderr)
                    (out / f'mutant-{n:08d}.error').write_text(error + '\n')
    print(f'mutants={idx} errors={failures} elapsed={time.monotonic()-start:.1f}s', flush=True)
    return 1 if failures else 0

if __name__ == '__main__':
    raise SystemExit(main())
