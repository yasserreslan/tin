#!/usr/bin/env python3
"""Run seeded Tin/Go differential and cross-target compiler checks."""
import argparse
from concurrent.futures import ThreadPoolExecutor
import hashlib
import os
from pathlib import Path
import platform
import shutil
import subprocess
import sys
import tempfile
import threading
import time

ROOT = Path(__file__).resolve().parents[2]
GEN = ROOT / 'tools/fuzz/gen.py'


def command_text(cmd):
    return ' '.join(str(x) for x in cmd)


def first_difference(left, right):
    a, b = left.splitlines(), right.splitlines()
    for i, (x, y) in enumerate(zip(a, b), 1):
        if x != y:
            return f'line {i}: Tin={x!r}; oracle={y!r}'
    if len(a) != len(b):
        i = min(len(a), len(b)) + 1
        return f'line {i}: Tin={a[i-1] if i <= len(a) else "<EOF>"!r}; oracle={b[i-1] if i <= len(b) else "<EOF>"!r}'
    return 'exit status differs'


def run(cmd, timeout, env=None):
    try:
        p = subprocess.run(cmd, cwd=ROOT, text=True, stdout=subprocess.PIPE,
                           stderr=subprocess.PIPE, timeout=timeout, env=env)
        return p.returncode, p.stdout, p.stderr, None
    except subprocess.TimeoutExpired as e:
        out = e.stdout or ''
        err = e.stderr or ''
        if isinstance(out, bytes): out = out.decode(errors='replace')
        if isinstance(err, bytes): err = err.decode(errors='replace')
        return None, out, err, 'timeout'


class Campaign:
    def __init__(self, args):
        self.args = args
        self.compiler = Path(args.compiler).resolve() if args.compiler else ROOT / 'bin/tinc'
        self.go = shutil.which('go')
        self.seed_lock = threading.Lock()
        self.next_seed = args.seed
        self.count = 0
        self.failures = set()
        self.lock = threading.Lock()
        self.started = time.monotonic()
        self.host = platform.machine().lower()
        self.targets = [t for t in ('linux-arm64', 'linux-amd64')
                        if not ((t.endswith('arm64') and self.host in ('aarch64', 'arm64')) or
                                (t.endswith('amd64') and self.host in ('x86_64', 'amd64')))]

    def seed(self):
        with self.seed_lock:
            value = self.next_seed
            self.next_seed += 1
            return value

    def record(self, kind, detail, seed, work, files, outputs, commands):
        signature = hashlib.sha256(f'{kind}|{detail}'.encode()).hexdigest()[:16]
        with self.lock:
            if signature in self.failures:
                return
            self.failures.add(signature)
        dest = self.args.out / signature
        dest.mkdir(parents=True, exist_ok=True)
        for source, name in files:
            if source.exists(): shutil.copy2(source, dest / name)
        (dest / 'outputs.txt').write_text('\n'.join(outputs))
        (dest / 'commands.txt').write_text('\n'.join(command_text(c) for c in commands) + '\n')
        (dest / 'signature.txt').write_text(f'{kind}: {detail}\nseed: {seed}\n')
        print(f'FAIL {signature} seed={seed} {kind}: {detail}', flush=True)

    def check(self, seed):
        with tempfile.TemporaryDirectory(prefix=f'tin-fuzz-{os.getpid()}-{seed}-') as temp:
            work = Path(temp)
            tin, gofile = work / 'prog.tin', work / 'prog.go'
            gen_cmd = [sys.executable, str(GEN), '--seed', str(seed), '--size', str(self.args.size), '--tin', str(tin), '--go', str(gofile)]
            rc, out, err, timeout = run(gen_cmd, 20)
            with self.lock:
                self.count += 1
            if timeout or rc:
                self.record('generator failure', err or out or f'exit {rc}', seed, work, [], [out, err], [gen_cmd]); return
            if not self.compiler.is_file():
                self.record('compiler unavailable', str(self.compiler), seed, work, [(tin, 'prog.tin'), (gofile, 'prog.go')], [], [gen_cmd]); return
            native = work / 'tin-native'
            build_cmd = [str(self.compiler), '-o', str(native), str(tin)]
            rc, out, err, timeout = run(build_cmd, 20)
            if timeout or rc:
                detail = 'timeout' if timeout else ('signal ' + str(-rc) if rc < 0 else f'exit {rc}: {err.strip()}')
                if 'E990 INTERNAL' in err: detail = 'E990 INTERNAL'
                self.record('compiler failure', detail, seed, work, [(tin, 'prog.tin'), (gofile, 'prog.go')], [out, err], [gen_cmd, build_cmd]); return
            rc, tout, terr, timeout = run([str(native)], 20)
            if timeout or rc:
                detail = 'timeout' if timeout else ('signal ' + str(-rc) if rc < 0 else f'program exit {rc}: {terr.strip()}')
                self.record('program failure', detail, seed, work, [(tin, 'prog.tin'), (gofile, 'prog.go')], [tout, terr], [build_cmd, [str(native)]]); return
            if 'go' in self.args.oracles and self.go:
                go_cmd = [self.go, 'run', str(gofile)]
                grc, gout, gerr, gtimeout = run(go_cmd, 30)
                if gtimeout or grc:
                    self.record('Go oracle failure', 'timeout' if gtimeout else f'exit {grc}: {gerr.strip()}', seed, work, [(tin, 'prog.tin'), (gofile, 'prog.go')], [tout, terr, gout, gerr], [build_cmd, [str(native)], go_cmd]); return
                if tout != gout:
                    self.record('Go mismatch', first_difference(tout, gout), seed, work, [(tin, 'prog.tin'), (gofile, 'prog.go')], [f'Tin stdout:\n{tout}', f'Tin stderr:\n{terr}', f'Go stdout:\n{gout}', f'Go stderr:\n{gerr}'], [build_cmd, [str(native)], go_cmd]); return
            if 'cross' in self.args.oracles:
                if sys.platform != 'linux':
                    return
                for target in self.targets:
                    cross = work / ('tin-' + target)
                    cross_cmd = [str(self.compiler), '-target', target, '-o', str(cross), str(tin)]
                    crc, cout, cerr, ctimeout = run(cross_cmd, 20)
                    if ctimeout or crc:
                        self.record('cross compiler failure', f'{target}: ' + ('timeout' if ctimeout else f'exit {crc}: {cerr.strip()}'), seed, work, [(tin, 'prog.tin'), (gofile, 'prog.go')], [cout, cerr], [cross_cmd]); return
                    runner = []
                    if target.endswith('arm64') and self.host not in ('aarch64', 'arm64'):
                        qemu = shutil.which('qemu-aarch64')
                        if qemu: runner = [qemu]
                        elif shutil.which('docker'):
                            runner = ['docker', 'run', '--rm', '-v', f'{work}:/w', 'arm64v8/debian', '/w/' + cross.name]
                        else:
                            print(f'SKIP cross {target}: no native runner, qemu-user, or Docker', flush=True); continue
                    elif target.endswith('amd64') and self.host not in ('x86_64', 'amd64'):
                        qemu = shutil.which('qemu-x86_64')
                        if qemu: runner = [qemu]
                        else:
                            print(f'SKIP cross {target}: no native runner or qemu-user', flush=True); continue
                    run_cmd = runner + [str(cross)]
                    xrc, xout, xerr, xtimeout = run(run_cmd, 20)
                    if xtimeout or xrc:
                        self.record('cross program failure', f'{target}: ' + ('timeout' if xtimeout else f'exit {xrc}: {xerr.strip()}'), seed, work, [(tin, 'prog.tin'), (gofile, 'prog.go')], [xout, xerr], [cross_cmd, run_cmd]); return
                    if xout != tout:
                        self.record('cross mismatch', f'{target}: {first_difference(tout, xout)}', seed, work, [(tin, 'prog.tin'), (gofile, 'prog.go')], [f'native stdout:\n{tout}', f'{target} stdout:\n{xout}'], [build_cmd, [str(native)], cross_cmd, run_cmd]); return

    def worker(self):
        while time.monotonic() - self.started < self.args.seconds:
            self.check(self.seed())


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--oracle', default='go')
    p.add_argument('--seconds', type=int, default=600)
    p.add_argument('--jobs', type=int, default=1)
    p.add_argument('--out', type=Path, default=Path('bin/fuzz'))
    p.add_argument('--compiler', default=os.environ.get('TIN_COMPILER'))
    p.add_argument('--seed', type=int, default=1)
    p.add_argument('--size', type=int, default=12)
    a = p.parse_args()
    a.oracles = set(a.oracle.split(','))
    unknown = a.oracles - {'go', 'cross', 'noopt'}
    if unknown or a.seconds < 1 or a.jobs < 1:
        p.error('invalid oracle, seconds, or jobs')
    if not GEN.is_file():
        print(f'ERROR generator is not present: {GEN} (provided by the generator PR)', file=sys.stderr)
        return 2
    a.out = a.out.resolve(); a.out.mkdir(parents=True, exist_ok=True)
    c = Campaign(a)
    if 'go' in a.oracles and not c.go:
        print('SKIP Go oracle: go is not installed', flush=True)
    if 'noopt' in a.oracles:
        print('SKIP noopt oracle: tinc -noopt is not available in this compiler', flush=True)
    if 'cross' in a.oracles and sys.platform != 'linux':
        print('SKIP cross oracle: Linux is required to execute Linux targets', flush=True)
    with ThreadPoolExecutor(max_workers=a.jobs) as pool:
        list(pool.map(lambda _: c.worker(), range(a.jobs)))
    elapsed = time.monotonic() - c.started
    print(f'completed {c.count} seeds in {elapsed:.1f}s; unique failures {len(c.failures)}', flush=True)
    return 1 if c.failures else 0


if __name__ == '__main__':
    raise SystemExit(main())
