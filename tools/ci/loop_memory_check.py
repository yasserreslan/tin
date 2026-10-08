#!/usr/bin/env python3
"""Loop iterations give their memory back (#633): binary-trees (bench/v2/bintrees.tin), which
builds and checks each depth's trees inside one loop iteration, peaks within 3x of its Go twin's
resident size, and the plain-program loop of the issue (5 million steps of a struct and a
string) under 16 MiB, and the loop bodies of #714 (an inlined call that allocates, a slice literal,
an inner loop) under 16 MiB each. Linux only: peak RSS is the child's ru_maxrss as /usr/bin/time reports it."""
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
from suite import ROOT

LOOP = '''package main

import "say"

type Row struct {
	id i64
	name str
}

fn parse(i i64) Row {
	return Row{id: i, name: "row {i}"}
}

fn main() {
	mut total = 0
	for i in 0..5000000 {
		let r = parse(i)
		total += len(r.name)
	}
	say.Line(total)
}
'''

# Loop bodies that keep nothing but were not given back before #714: 3 million steps each.
BODIES = {
    'inlined mint.Itoa': ('let s = mint.Itoa(i)\n\t\tif len(s) > 0 {\n\t\t\ttotal += 1\n\t\t}', '3000000'),
    'str slice literal': ('let parts = []str{"a", mint.Itoa(i)}\n\t\ttotal += len(parts[1])', '19888890'),
    'make and store': ('let parts = make([]str, 2)\n\t\tparts[1] = mint.Itoa(i)\n\t\ttotal += len(parts[1])', '19888890'),
    'inner loop': ('let s = mint.Itoa(i)\n\t\tfor c in s {\n\t\t\ttotal += i64(c)\n\t\t}', '1038666720'),
}


def body_program(body):
    return ('package main\n\nimport "mint"\nimport "say"\n\nfn main() {\n\tmut total = 0\n'
            '\tfor i in 0..3000000 {\n\t\t' + body + '\n\t}\n\tsay.Line(total)\n}\n')


def peak_kib(argv):
    """The exit status, stdout and peak resident KiB of argv, run under /usr/bin/time."""
    result = subprocess.run(['/usr/bin/time', '-f', 'PEAK %M'] + argv, capture_output=True, timeout=300)
    lines = [l for l in result.stderr.decode(errors='replace').split('\n') if l.startswith('PEAK ')]
    assert lines, result.stderr[-500:]
    return result.returncode, result.stdout, int(lines[-1].split()[1])


def main():
    if not sys.platform.startswith('linux'):
        print('SKIP loop memory: peak RSS is measured on Linux')
        return
    if not os.path.exists('/usr/bin/time') or shutil.which('go') is None:
        if os.environ.get('CI'):
            raise SystemExit('loop memory: needs /usr/bin/time and go on Linux CI runners')
        print('SKIP loop memory: needs /usr/bin/time and go')
        return
    with tempfile.TemporaryDirectory(prefix='tin-loop-memory-') as d:
        out = Path(d)
        env = dict(os.environ, TIN_ROOT=str(ROOT))
        subprocess.run([str(ROOT / 'bin/tinc'), '-o', str(out / 'bintrees'), 'bench/v2/bintrees.tin'],
                       check=True, cwd=ROOT, env=env, timeout=300)
        subprocess.run(['go', 'build', '-o', str(out / 'bintrees_go'), 'bench/v2/bintrees.go'],
                       check=True, cwd=ROOT, timeout=300)
        (out / 'loop.tin').write_text(LOOP)
        subprocess.run([str(ROOT / 'bin/tinc'), '-o', str(out / 'loop'), str(out / 'loop.tin')],
                       check=True, cwd=ROOT, env=env, timeout=300)
        code, tin_out, tin = peak_kib([str(out / 'bintrees')])
        assert code == 0, code
        code, go_out, go = peak_kib([str(out / 'bintrees_go')])
        assert code == 0, code
        assert tin_out == go_out, (tin_out[-300:], go_out[-300:])
        print(f'binary-trees depth 18: Tin peak {tin} KiB, Go peak {go} KiB, Tin/Go {tin / go:.2f}')
        assert tin <= 3 * go, f'binary-trees peaks at {tin} KiB, more than 3x Go ({go} KiB)'
        code, loop_out, loop = peak_kib([str(out / 'loop')])
        assert code == 0 and loop_out == b'53888890\n', (code, loop_out)
        print(f'plain loop of 5M structs and strings: peak {loop} KiB')
        assert loop < 16 * 1024, f'the plain loop peaks at {loop} KiB'
        for name, (body, want) in BODIES.items():
            (out / 'body.tin').write_text(body_program(body))
            subprocess.run([str(ROOT / 'bin/tinc'), '-o', str(out / 'body'), str(out / 'body.tin')],
                           check=True, cwd=ROOT, env=env, timeout=300)
            code, body_out, peak = peak_kib([str(out / 'body')])
            assert code == 0 and body_out == (want + '\n').encode(), (name, code, body_out)
            print(f'loop body {name}: peak {peak} KiB')
            assert peak < 16 * 1024, f'the loop body {name} peaks at {peak} KiB (#714)'
    print('PASS loop memory: binary-trees within 3x of Go, the plain loop and the #714 bodies under 16 MiB')


if __name__ == '__main__':
    main()
