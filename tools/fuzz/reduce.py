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
    # Differential artifacts may carry their signature explicitly.
    m = re.search(r'(?:mismatch|diff)[^\n]*line\s*(\d+)', text, re.I)
    if m: return 'mismatch:line:' + m.group(1)
    return None


class Reducer:
    def __init__(self, compiler: str, timeout: float, wanted: str, twin: Path | None):
        self.compiler, self.timeout, self.wanted, self.twin = compiler, timeout, wanted, twin
        self.deadline = time.monotonic() + 120
        self.calls = 0

    def preserves(self, tin: str, go: str | None) -> bool:
        if time.monotonic() > self.deadline: return False
        self.calls += 1
        with tempfile.TemporaryDirectory(prefix='tin-reduce-') as td:
            src = Path(td) / 'repro.tin'; src.write_text(tin)
            try:
                p = subprocess.run([self.compiler, '-S', str(src)], capture_output=True, text=True, errors='surrogateescape', timeout=self.timeout, env={**os.environ, 'TIN_ROOT': str(Path.cwd())})
                sig = signature(p.stdout + '\n' + p.stderr, p.returncode)
            except subprocess.TimeoutExpired:
                sig = 'hang'
            except OSError:
                return False
            if sig != self.wanted: return False
            if go is not None and self.twin:
                gp = Path(td) / 'repro.go'; gp.write_text(go)
                try:
                    q = subprocess.run(['go', 'run', str(gp)], capture_output=True, text=True, errors='surrogateescape', timeout=self.timeout)
                except (OSError, subprocess.TimeoutExpired):
                    return True
                # Keep twin text paired; a successful Go run is required when available.
                return q.returncode == 0
            return True

    def declarations(self, text: str) -> str:
        lines = text.splitlines(keepends=True)
        starts = [i for i, line in enumerate(lines) if i > 0 and line and not line[0].isspace() and re.match(r'(?:fn|type|const|shape|use|on|let|mut)\b', line)]
        # Try removing whole top-level declarations while retaining package/import headers.
        changed = True
        while changed and time.monotonic() < self.deadline:
            changed = False
            for n, begin in enumerate(starts):
                end = starts[n + 1] if n + 1 < len(starts) else len(lines)
                candidate = ''.join(lines[:begin] + lines[end:])
                if candidate and self.preserves(candidate, None):
                    lines = candidate.splitlines(keepends=True)
                    starts = [i for i, line in enumerate(lines) if i > 0 and line and not line[0].isspace() and re.match(r'(?:fn|type|const|shape|use|on|let|mut)\b', line)]
                    changed = True
                    break
        return ''.join(lines)

    def chunks(self, text: str, stage: int) -> str:
        lines = text.splitlines(keepends=True)
        if len(lines) < 2: return text
        granularity = 2
        while len(lines) >= 2 and time.monotonic() < self.deadline:
            size = max(1, (len(lines) + granularity - 1) // granularity)
            changed = False
            for start in range(0, len(lines), size):
                candidate = lines[:start] + lines[start + size:]
                if candidate and self.preserves(''.join(candidate), None):
                    lines, granularity, changed = candidate, max(2, granularity - 1), True
                    break
            if not changed:
                if granularity >= len(lines): break
                granularity = min(len(lines), granularity * 2)
        return ''.join(lines)


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('artifact', type=Path)
    ap.add_argument('--out', required=True, type=Path)
    ap.add_argument('--compiler', default=os.environ.get('TINC', 'bin/tinc'))
    ap.add_argument('--signature')
    ap.add_argument('--timeout', type=float, default=10)
    a = ap.parse_args()
    tin = a.artifact if a.artifact.is_file() else a.artifact / 'repro.tin'
    if not tin.is_file():
        choices = sorted(a.artifact.glob('*.tin')) if a.artifact.is_dir() else []
        if not choices: ap.error(f'no .tin source found in {a.artifact}')
        tin = choices[0]
    source = tin.read_text(encoding='utf-8', errors='surrogateescape')
    twin = tin.with_suffix('.go')
    go = twin.read_text() if twin.is_file() else None
    meta = (a.artifact / 'signature.txt').read_text() if a.artifact.is_dir() and (a.artifact / 'signature.txt').is_file() else ''
    # Run once to derive a stable signature where a failure artifact has no signature file.
    if a.signature:
        wanted = a.signature
    else:
        try:
            p = subprocess.run([a.compiler, '-S', str(tin)], capture_output=True, text=True, errors='surrogateescape', timeout=a.timeout)
            wanted = signature(p.stdout + '\n' + p.stderr, p.returncode)
        except subprocess.TimeoutExpired:
            wanted = 'hang'
        except OSError as e:
            ap.error(f'cannot run compiler: {e}')
        wanted = meta.strip() or wanted
    if not wanted: ap.error('could not determine failure signature; pass --signature')
    r = Reducer(a.compiler, a.timeout, wanted, twin if go is not None and shutil.which('go') else None)
    # Three reduction stages: declaration-sized chunks, statement/block chunks, then token simplification.
    reduced = r.declarations(source)
    reduced = r.chunks(reduced, 2)
    literals = ('0', '1', 'false', 'true', 'nil', '""')
    for literal in literals:
        if time.monotonic() >= r.deadline: break
        for match in list(re.finditer(r'\b(?:\d+|true|false|null)\b', reduced)):
            candidate = reduced[:match.start()] + literal + reduced[match.end():]
            if len(candidate) < len(reduced) and r.preserves(candidate, go): reduced = candidate
    a.out.parent.mkdir(parents=True, exist_ok=True)
    a.out.write_text(reduced, encoding='utf-8', errors='surrogateescape')
    if go is not None:
        twin_out = a.out.with_suffix('.go'); twin_out.write_text(go, encoding='utf-8')
    print(f'signature={wanted} attempts={r.calls} lines={len(reduced.splitlines())} out={a.out}')
    return 0

if __name__ == '__main__':
    raise SystemExit(main())
