#!/usr/bin/env python3
"""Signature-preserving delta reducer for Tin fuzz artifacts."""
from __future__ import annotations

import argparse
import os
import re
import shutil
import subprocess
import tempfile
import time
from pathlib import Path

DECL = re.compile(r'(?m)^(?:fn|type|const|shape|use|on)\b')


def signature(text: str, code: int, timeout: bool = False) -> str | None:
    if timeout: return 'hang'
    if code < 0: return f'signal:{-code}'
    if code == 2:
        if re.search(r'segmentation fault|access violation', text, re.I): return 'crash:segmentation-fault'
        if re.search(r'panic:', text, re.I): return 'crash:panic'
        return 'crash:exit2'
    m = re.search(r'\bE\d{3}\b', text)
    if m: return 'diagnostic:' + m.group()
    return None


def difference_signature(tin: str, go: str) -> str | None:
    a, b = tin.splitlines(), go.splitlines()
    for i in range(max(len(a), len(b))):
        av = a[i] if i < len(a) else '<EOF>'
        bv = b[i] if i < len(b) else '<EOF>'
        if av != bv: return f'mismatch:line:{i + 1}'
    return None


def read_text(path: Path) -> str:
    return path.read_text(encoding='utf-8', errors='surrogateescape')


class Reducer:
    def __init__(self, compiler: str, timeout: float, wanted: str, twin: Path | None, workdir: Path):
        self.compiler, self.timeout, self.wanted, self.twin = compiler, timeout, wanted, twin
        self.workdir = workdir
        self.deadline = time.monotonic() + 120
        self.calls = 0
        self.go_available = twin is not None and shutil.which('go') is not None

    def evaluate(self, tin: str, go: str | None) -> str | None:
        if time.monotonic() > self.deadline: return None
        self.calls += 1
        try:
            with tempfile.TemporaryDirectory(prefix='tin-reduce-', dir=self.workdir) as td:
                td = Path(td)
                src = td / 'repro.tin'; src.write_text(tin, encoding='utf-8', errors='surrogateescape')
                binary = td / 'repro'
                p = subprocess.run([self.compiler, '-o', str(binary), str(src)], cwd=self.workdir,
                    capture_output=True, text=True, errors='surrogateescape', timeout=self.timeout,
                    env={**os.environ, 'TIN_ROOT': str(Path.cwd())})
                if p.returncode != 0:
                    return signature(p.stdout + '\n' + p.stderr, p.returncode)
                try:
                    run = subprocess.run([str(binary)], cwd=self.workdir, capture_output=True, text=True,
                        errors='surrogateescape', timeout=self.timeout)
                except subprocess.TimeoutExpired:
                    return 'hang'
                if run.returncode != 0:
                    return signature(run.stdout + '\n' + run.stderr, run.returncode)
                if not self.go_available or go is None:
                    return None
                gp = td / 'repro.go'; gp.write_text(go, encoding='utf-8')
                try:
                    q = subprocess.run(['go', 'run', str(gp)], cwd=self.workdir, capture_output=True,
                        text=True, errors='surrogateescape', timeout=self.timeout)
                except (OSError, subprocess.TimeoutExpired):
                    return None
                if q.returncode != 0: return None
                return difference_signature(run.stdout, q.stdout)
        except subprocess.TimeoutExpired:
            return 'hang'
        except OSError:
            return None

    def preserves(self, tin: str, go: str | None) -> bool:
        if time.monotonic() > self.deadline: return False
        found = self.evaluate(tin, go)
        return found == self.wanted

    def declarations(self, text: str, go: str | None) -> str:
        lines = text.splitlines(keepends=True)
        while time.monotonic() < self.deadline:
            starts = [i for i, line in enumerate(lines) if i > 0 and line and not line[0].isspace() and re.match(r'(?:fn|type|const|shape|use|on|let|mut)\b', line)]
            changed = False
            for n, begin in enumerate(starts):
                end = starts[n + 1] if n + 1 < len(starts) else len(lines)
                candidate = ''.join(lines[:begin] + lines[end:])
                if candidate and self.preserves(candidate, go):
                    lines = candidate.splitlines(keepends=True)
                    changed = True
                    break
            if not changed: break
        return ''.join(lines)

    def chunks(self, text: str, check) -> str:
        lines = text.splitlines(keepends=True)
        granularity = 2
        while len(lines) >= 2 and time.monotonic() < self.deadline:
            size = max(1, (len(lines) + granularity - 1) // granularity)
            changed = False
            for start in range(0, len(lines), size):
                candidate = lines[:start] + lines[start + size:]
                if candidate and check(''.join(candidate)):
                    lines, granularity, changed = candidate, max(2, granularity - 1), True
                    break
            if not changed:
                if granularity >= len(lines): break
                granularity = min(len(lines), granularity * 2)
        return ''.join(lines)

    def simplify(self, text: str, check) -> str:
        for pattern, replacement in ((r'\b\d+\b', '0'), (r'\b\d+\b', '1'), (r'"(?:\\.|[^"\\])*"', '""')):
            for match in list(re.finditer(pattern, text)):
                candidate = text[:match.start()] + replacement + text[match.end():]
                if len(candidate) < len(text) and check(candidate): text = candidate
                if time.monotonic() >= self.deadline: return text
        return text


def initial_signature(compiler: str, tin: Path, twin: Path | None, timeout: float, workdir: Path) -> str | None:
    if twin and shutil.which('go'):
        try:
            with tempfile.TemporaryDirectory(prefix='tin-reduce-probe-', dir=workdir) as td:
                td = Path(td); src = td / 'repro.tin'; src.write_text(read_text(tin), encoding='utf-8', errors='surrogateescape')
                binary = td / 'repro'
                p = subprocess.run([compiler, '-o', str(binary), str(src)], cwd=workdir, capture_output=True, text=True,
                    errors='surrogateescape', timeout=timeout, env={**os.environ, 'TIN_ROOT': str(Path.cwd())})
                if p.returncode: return signature(p.stdout + '\n' + p.stderr, p.returncode)
                run = subprocess.run([str(binary)], cwd=workdir, capture_output=True, text=True, errors='surrogateescape', timeout=timeout)
                if run.returncode: return signature(run.stdout + '\n' + run.stderr, run.returncode)
                q = subprocess.run(['go', 'run', str(twin)], cwd=workdir, capture_output=True, text=True, errors='surrogateescape', timeout=timeout)
                return difference_signature(run.stdout, q.stdout) if q.returncode == 0 else None
        except subprocess.TimeoutExpired: return 'hang'
        except OSError: return None
    try:
        p = subprocess.run([compiler, '-S', str(tin)], cwd=workdir, capture_output=True, text=True, errors='surrogateescape',
            timeout=timeout, env={**os.environ, 'TIN_ROOT': str(Path.cwd())})
        return signature(p.stdout + '\n' + p.stderr, p.returncode)
    except subprocess.TimeoutExpired: return 'hang'
    except OSError: return None


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('artifact', type=Path)
    ap.add_argument('--out', required=True, type=Path)
    ap.add_argument('--compiler', default=os.environ.get('TINC', 'bin/tinc'))
    ap.add_argument('--signature')
    ap.add_argument('--timeout', type=float, default=10)
    a = ap.parse_args()
    artifact = a.artifact.resolve()
    tin = artifact if artifact.is_file() else artifact / 'repro.tin'
    if not tin.is_file():
        choices = sorted(artifact.glob('*.tin')) if artifact.is_dir() else []
        if not choices: ap.error(f'no .tin source found in {artifact}')
        tin = choices[0]
    source = read_text(tin)
    twin = tin.with_suffix('.go')
    go = read_text(twin) if twin.is_file() else None
    compiler = str(Path(a.compiler).resolve())
    workdir = tin.parent
    meta = read_text(artifact / 'signature.txt').strip() if artifact.is_dir() and (artifact / 'signature.txt').is_file() else ''
    wanted = a.signature
    if not wanted:
        wanted = initial_signature(compiler, tin, twin if go is not None else None, a.timeout, workdir)
    if meta:
        code = re.search(r'\bE\d{3}\b', meta)
        mismatch = re.search(r'(?:mismatch|diff)[^\n]*line\s*(\d+)', meta, re.I)
        sig = signature(meta, 1) if code else (f'mismatch:line:{mismatch.group(1)}' if mismatch else None)
        wanted = sig or wanted
    if not wanted: ap.error('could not determine failure signature; pass --signature')
    r = Reducer(compiler, a.timeout, wanted, twin if go is not None else None, workdir)
    if go is not None and not r.go_available:
        print('go unavailable; skipping differential twin reduction')
        go = None
        # Retain a compiler failure signature when Go is unavailable.
        wanted = initial_signature(compiler, tin, None, a.timeout, workdir) or wanted
        r.wanted = wanted
    original_go = go
    # Stage 1: remove top-level declarations from Tin.
    reduced = r.declarations(source, go)
    # Stage 2: remove statements and blocks from Tin, then the Go twin while holding Tin fixed.
    reduced = r.chunks(reduced, lambda candidate: r.preserves(candidate, go))
    if go is not None:
        tin_fixed = reduced
        go = r.chunks(go, lambda candidate: r.preserves(tin_fixed, candidate))
    # Stage 3: simplify expressions in both texts while preserving the same signature.
    reduced = r.simplify(reduced, lambda candidate: r.preserves(candidate, go))
    if go is not None:
        tin_fixed = reduced
        go = r.simplify(go, lambda candidate: r.preserves(tin_fixed, candidate))
    a.out.parent.mkdir(parents=True, exist_ok=True)
    a.out.write_text(reduced, encoding='utf-8', errors='surrogateescape')
    if original_go is not None:
        a.out.with_suffix('.go').write_text(go or original_go, encoding='utf-8', errors='surrogateescape')
    print(f'signature={wanted} attempts={r.calls} lines={len(reduced.splitlines())} out={a.out}')
    return 0

if __name__ == '__main__':
    raise SystemExit(main())
