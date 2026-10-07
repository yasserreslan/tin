#!/usr/bin/env python3
"""ledger streaming and limits (#572): reading a 1 GiB CSV file through ledger.NewStream, with
each Read running in an arena, keeps a flat footprint, and a line longer than the reader's
window fails with fault.LimitExceeded, so hostile input cannot grow the reader.

Peak RSS is Linux acceptance (toolchain/docs/CI.md); macOS runs everything and reports the numbers.
"""
import os
import subprocess
import sys

from suite import ROOT

MIB = 1024 * 1024


def make_csv(path, records):
    """Writes the file and returns the expected (records, field bytes) counts."""
    pad = "x" * 100
    total = 0
    with open(path, "wb") as f:
        for i in range(records):
            row = []
            for j in range(8):
                v = "f%07d-%02d-%s" % (i, j, pad)
                if i % 1000 == 0 and j == 0:
                    v += ",tail"
                    row.append('"%s"' % v)
                elif i % 997 == 0 and j == 1:
                    v = "line%d\nsecond" % i
                    row.append('"%s"' % v)
                else:
                    row.append(v)
                total += len(v)
            f.write((",".join(row) + "\r\n").encode())
    return records, total


def run(exe, path):
    """Runs the fixture and returns (output, peak RSS bytes)."""
    env = dict(os.environ, LEDGER_STREAM_FILE=str(path))
    p = subprocess.Popen([str(exe)], stdout=subprocess.PIPE, env=env)
    out = p.stdout.read()
    _, status, usage = os.wait4(p.pid, 0)
    p.stdout.close()
    p.returncode = os.waitstatus_to_exitcode(status)
    assert p.returncode == 0, (p.returncode, out)
    kb = usage.ru_maxrss
    return out, (kb if sys.platform == "darwin" else kb * 1024)


def main():
    out = ROOT / "bin/ci/ledger"
    out.mkdir(parents=True, exist_ok=True)
    exe = out / "ledger_stream"
    subprocess.run([str(ROOT / "bin/tinc"), "-edition", "1", "-o", str(exe), "tools/ci/fixtures/ledger_stream.tin"],
                   cwd=ROOT, env=dict(os.environ, TIN_ROOT=str(ROOT)), check=True)

    path = out / "stream.csv"
    records, field_bytes = make_csv(path, 1150000)
    got, peak = run(exe, path)
    want = b"records %d bytes %d\n" % (records, field_bytes)
    assert got == want, (got[:200], want[:200])
    print("1 GiB CSV: %d records, %d field bytes; peak RSS %.1f MiB" % (records, field_bytes, peak / MIB))
    if sys.platform == "linux":
        assert peak < 64 * MIB, "peak RSS %d over 64 MiB" % peak
        print("streaming peak RSS stays under 64 MiB (Linux)")
    path.unlink()

    long = out / "long.csv"
    long.write_bytes(b"a" * 80000 + b"\nok,ok\n")
    got, peak = run(exe, long)
    assert got.startswith(b"fault ledger: line longer than 65536 bytes"), got
    print("an 80 KiB line stops inside the 64 KiB window: %s" % got.decode().strip())
    long.unlink()


if __name__ == "__main__":
    main()
