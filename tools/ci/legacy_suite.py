#!/usr/bin/env python3
"""Run the legacy-syntax tests (tests/*.tin, tests/errors/*.tin) against the self-hosted compiler.

A program under tests/ carries its own expectations in comments: each `// expect: TEXT` is one
line of stdout and `// exit: N` is the exit status (default 0). A program under tests/errors/
starts with `// error: MESSAGE` and must be rejected with MESSAGE in the compiler's output.

Modes:
  native  tinc -o EXE lib/std.tin TEST            the compiler writes the executable
  asm     tinc -S lib/std.tin TEST, then cc       the compiler prints Darwin assembly, cc links it
                                                  (macOS only: the assembly is Mach-O)

A crash or timeout never counts as passing, and a compiler crash is not a rejection.
"""
import argparse
from concurrent.futures import ThreadPoolExecutor
import difflib
import os
from pathlib import Path
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[2]
TIMEOUT = 60


def execute(command, *, cwd=ROOT, timeout=TIMEOUT):
    """Returns (exit status, stdout, stderr); 124 and a note on stderr for a timeout."""
    try:
        result = subprocess.run(command, cwd=cwd, capture_output=True, timeout=timeout)
        return result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired as exc:
        return 124, exc.stdout or b'', (exc.stderr or b'') + b'\nCI timeout\n'


def expectations(source):
    """The stdout and exit status a test program asks for."""
    out, exit_status = [], 0
    for line in source.split('\n'):
        i = line.find('// expect:')
        if i >= 0:
            text = line[i + len('// expect:'):]
            out.append((text[1:] if text.startswith(' ') else text) + '\n')
        i = line.find('// exit: ')
        if i >= 0:
            try:
                exit_status = int(line[i + len('// exit: '):].strip())
            except ValueError:
                exit_status = 0
    return ''.join(out), exit_status


def build(compiler, mode, sources, exe, root=ROOT):
    """Compiles sources into exe. Returns (exit status, compiler output as text)."""
    if mode == 'native':
        code, out, err = execute([str(compiler), '-o', str(exe)] + sources, cwd=root)
        return code, (out + err).decode(errors='replace')
    code, asm, err = execute([str(compiler), '-S'] + sources, cwd=root)
    if code != 0:
        return code, (asm + err).decode(errors='replace')
    asm_path = Path(str(exe) + '.s')
    asm_path.write_bytes(asm)
    code, out, err = execute(['cc', '-o', str(exe), str(asm_path)], cwd=root)
    return code, (out + err).decode(errors='replace')


def run_program(compiler, mode, path, work, root=ROOT):
    """Returns None if the program built and behaved as it asked, else a description."""
    want_out, want_exit = expectations(path.read_text(errors='replace'))
    exe = work / path.stem
    code, output = build(compiler, mode, ['lib/std.tin', str(path.relative_to(root))], exe, root)
    if code != 0:
        return f'build failed (exit {code}):\n{output[:4000]}'
    code, out, err = execute([str(exe)], cwd=root)
    problems = []
    if code != want_exit:
        problems.append(f'exit status {code}, want {want_exit}')
    if out.decode(errors='replace') != want_out:
        diff = ''.join(difflib.unified_diff(want_out.splitlines(True),
                                            out.decode(errors='replace').splitlines(True),
                                            fromfile='expected', tofile='actual'))
        problems.append('stdout differs:\n' + diff[:4000])
    return '\n'.join(problems) or None


def run_error_case(compiler, mode, path, work, root=ROOT):
    """Returns None if the compiler rejected the program with the promised message."""
    first = path.read_text(errors='replace').split('\n', 1)[0]
    if not first.startswith('// error: '):
        return 'first line must be `// error: MESSAGE`'
    want = first[len('// error: '):]
    code, output = build(compiler, mode, ['lib/std.tin', str(path.relative_to(root))], work / path.stem, root)
    if code == 0:
        return f'compiled successfully, want an error containing {want!r}'
    if code != 1:
        return f'compiler did not reject cleanly (exit {code}):\n{output[:4000]}'
    if want not in output:
        return f'error {output.strip()!r} does not contain {want!r}'
    return None


def run(compiler, mode='native', root=ROOT):
    programs = sorted((root / 'tests').glob('*.tin'))
    errors = sorted((root / 'tests/errors').glob('*.tin'))
    if not programs:
        print('FAIL: no legacy test programs found')
        return False
    output = root / 'bin/ci'
    output.mkdir(parents=True, exist_ok=True)
    cases = [(p, run_program) for p in programs] + [(p, run_error_case) for p in errors]
    # A fresh directory per run: a failed build can never leave a stale executable to run.
    with tempfile.TemporaryDirectory(prefix='legacy-', dir=output) as work:
        work = Path(work)

        def one(case):
            path, check = case
            sub = work / ('e_' if check is run_error_case else 'p_')
            sub.mkdir(exist_ok=True)
            return path, check(compiler, mode, path, sub, root)

        with ThreadPoolExecutor(max_workers=os.cpu_count() or 4) as pool:
            results = list(pool.map(one, cases))
    failed = 0
    for path, problem in results:
        name = path.relative_to(root / 'tests')
        if problem is None:
            print('PASS', name)
        else:
            failed += 1
            print(f'FAIL {name}: {problem}')
    print(f'legacy suite ({mode}): {len(results) - failed} passed, {failed} failed')
    return failed == 0


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument('compiler', nargs='?', default='bin/tinc')
    parser.add_argument('--mode', choices=['native', 'asm'], default='native')
    args = parser.parse_args()
    return 0 if run(Path(args.compiler).resolve(), args.mode) else 1


if __name__ == '__main__':
    sys.exit(main())
