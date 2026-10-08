#!/usr/bin/env python3
"""Compare pack and the small encodings with Go's encoding/binary, encoding/base32 and
encoding/ascii85.

A generated corpus of values and byte strings (a fixed seed, so both sides see the same cases) is
fed to bench/ref/pack and tools/ci/fixtures/pack.tin; the check compares their lines: the packed
forms of u64 values, the u64 reads and varints of byte strings (including truncated and
over-long encodings), and the base32/ascii85 encodings, decodes and round trips.
"""
import random
import subprocess
import tempfile
from pathlib import Path

from suite import ROOT

SEED = 740
VALUES = 4000
BYTES = 4000


def varint_bytes(v):
    """Go's PutUvarint."""
    out = bytearray()
    while v >= 0x80:
        out.append((v & 0x7f) | 0x80)
        v >>= 7
    out.append(v)
    return bytes(out)


def zigzag(v):
    return (v << 1) ^ (v >> 63) if v >= 0 else ((v << 1) ^ (v >> 63)) & ((1 << 64) - 1)


def corpus():
    rng = random.Random(SEED)
    lines = []
    fixed = [0, 1, 2, 127, 128, 255, 256, 0xffff, 0x10000, 2**32 - 1, 2**32, 2**63 - 1, 2**63,
             2**64 - 1, 300, 16384]
    for v in fixed:
        lines.append("P " + v.to_bytes(8, "big").hex())
    while len(lines) < VALUES:
        v = rng.getrandbits(64)
        if rng.random() < 0.3:
            v = rng.getrandbits(rng.randint(1, 32))
        lines.append("P " + v.to_bytes(8, "big").hex())

    # Varint corpora: valid, truncated and over-long encodings, and arbitrary bytes.
    extra = []
    for _ in range(BYTES // 4):
        v = rng.getrandbits(rng.randint(1, 64))
        extra.append(varint_bytes(v))
        enc = varint_bytes(v)
        if len(enc) > 1:
            extra.append(enc[:rng.randint(1, len(enc) - 1)])
        sv = rng.randint(-(2**63), 2**63 - 1)
        extra.append(varint_bytes(zigzag(sv)))
    extra.append(b"\xff" * 10)
    extra.append(b"\xff" * 9 + b"\x02")
    extra.append(b"\x80" * 10)
    extra.append(b"")
    while len(extra) < BYTES:
        n = rng.randint(0, 12)
        extra.append(bytes(rng.randrange(256) for _ in range(n)))
    # The encodings get arbitrary bytes too: every length, padding in the middle, whitespace,
    # 'z' and '~' in ascii85, and invalid alphabet bytes.
    special = [b"", b"=" , b"====", b"A", b"A=", b"AB", b"ABC", b"ABCDE", b"MZXW6", b"MZXW6===",
               b"MZXW6====", b"=MZXW6", b"MZ=W6===", b"mzxw6===", b"7Z======", b"z", b"~>",
               b"<~9jqo~>", b"9jq o", b"9jqo\n", b"!!!!!", b"uuuuu", b"vvvvv", b"!!!!",
               b"\xff\xff\xff\xff\xff", b"9jqo~", b"z9jqo", b"\t\n 9jqo \r"]
    lines += ["U " + b.hex() for b in extra]
    lines += ["B " + b.hex() for b in special]
    while len([l for l in lines if l.startswith("B ")]) < 2000:
        n = rng.randint(0, 10)
        b = bytes(rng.choice(b"ABZ27=az\t\nz~!u\xff") for _ in range(n))
        lines.append("B " + b.hex())
    return lines


def run(exe, data):
    result = subprocess.run([str(exe)], input=data.encode(), capture_output=True, check=True,
                            cwd=ROOT, timeout=600)
    return result.stdout.decode().splitlines()


def corpus_line(lines, out_index):
    """The corpus line an output line came from: P and U print one line, B prints three."""
    n = 0
    for line in lines:
        n += 3 if line.startswith("B ") else 1
        if out_index < n:
            return line
    return "?"


def main():
    lines = corpus()
    data = "\n".join(lines) + "\n"
    directory = ROOT / "bin/ci/pack"
    directory.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="pack-", dir=directory) as tmp:
        work = Path(tmp)
        tin = work / "pack-tin"
        subprocess.run([str(ROOT / "bin/tinc"), "-o", str(tin), "tools/ci/fixtures/pack.tin"],
                       check=True, cwd=ROOT, env={"TIN_ROOT": str(ROOT)}, timeout=300)
        go = work / "pack-go"
        subprocess.run(["go", "build", "-o", str(go), "./bench/ref/pack"], check=True, cwd=ROOT,
                       timeout=300)
        want = run(go, data)
        got = run(tin, data)
        assert len(got) == len(want), f"pack: {len(got)} lines against Go's {len(want)}"
        bad = 0
        for i, (a, b) in enumerate(zip(want, got)):
            if a != b:
                print(f"line {i + 1} (corpus line {corpus_line(lines, i)}):")
                print(f"  Go:  {a}")
                print(f"  Tin: {b}")
                bad += 1
                if bad >= 10:
                    break
        assert bad == 0, f"pack: {bad} of {len(want)} lines differ"
        print(f"PASS pack: {len(want)} lines ({len(lines)} corpus cases) match Go's "
              f"encoding/binary, base32 and ascii85")


if __name__ == "__main__":
    main()
