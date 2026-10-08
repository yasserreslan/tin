#!/usr/bin/env python3
"""Compare the io helpers with Go's io.

A generated corpus of chunk patterns (a fixed seed, so both sides see the same cases) is fed to
bench/ref/io and tools/ci/fixtures/io.tin; both read the same deterministic byte sequence through
a reader that returns it in the given chunk sizes and print one line per case: ReadAll and Copy
with several buffer sizes, LimitReader, ReadFull, ReadAtLeast, CopyN and TeeReader. The check
compares the lines.
"""
import random
import subprocess
import tempfile
from pathlib import Path

from suite import ROOT

SEED = 736
CASES = 4000


def corpus():
    rng = random.Random(SEED)
    lines = []
    patterns = [[1], [2], [3], [1, 1], [1, 1, 1], [4, 1, 3], [8], [1, 7], [5, 5, 5],
                list(range(1, 9)), [9, 1, 1, 1], [2, 3, 5, 7, 11]]
    while len(patterns) < CASES // 4:
        n = rng.randint(1, 6)
        patterns.append([rng.randint(1, 8) for _ in range(n)])
    for chunks in patterns:
        spec = ",".join(str(c) for c in chunks)
        total = sum(chunks)
        lines.append(f"R {spec}")
        for k in {0, 1, max(0, total // 2), total, total + 5}:
            lines.append(f"L {spec} {k}")
        for k in {0, 1, max(0, total // 2), total, total + 3}:
            lines.append(f"F {spec} {k}")
            lines.append(f"N {spec} {k}")
        lines.append(f"T {spec}")
    return lines


def run(exe, data):
    result = subprocess.run([str(exe)], input=data.encode(), capture_output=True, check=True,
                            cwd=ROOT, timeout=600)
    return result.stdout.decode().splitlines()


def main():
    lines = corpus()
    data = "\n".join(lines) + "\n"
    directory = ROOT / "bin/ci/io"
    directory.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="io-", dir=directory) as tmp:
        work = Path(tmp)
        tin = work / "io-tin"
        subprocess.run([str(ROOT / "bin/tinc"), "-o", str(tin), "tools/ci/fixtures/io.tin"],
                       check=True, cwd=ROOT, env={"TIN_ROOT": str(ROOT)}, timeout=300)
        go = work / "io-go"
        subprocess.run(["go", "build", "-o", str(go), "./bench/ref/io"], check=True, cwd=ROOT,
                       timeout=300)
        want = run(go, data)
        got = run(tin, data)
        assert len(got) == len(want), f"io: {len(got)} lines against Go's {len(want)}"
        bad = 0
        for i, (a, b) in enumerate(zip(want, got)):
            if a != b:
                print(f"line {i + 1} (corpus line {lines[i] if i < len(lines) else '?'}):")
                print(f"  Go:  {a}")
                print(f"  Tin: {b}")
                bad += 1
                if bad >= 10:
                    break
        assert bad == 0, f"io: {bad} of {len(want)} lines differ"
        print(f"PASS io: {len(want)} lines ({len(lines)} corpus cases) match Go's io")


if __name__ == "__main__":
    main()
