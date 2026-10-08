#!/usr/bin/env python3
"""Compare mime, quoted-printable and multipart with Go's mime packages.

A generated (and hand-written) corpus of media type strings, quoted-printable texts and multipart
bodies is fed to bench/ref/mime and tools/ci/fixtures/mime.tin; the check compares the lines: the
parsed media types and parameters, the decoded and re-encoded quoted-printable texts, and the
parts of each multipart body (name, filename, content type, decoded body).
"""
import random
import subprocess
import tempfile
from pathlib import Path

from suite import ROOT

SEED = 735

MEDIA = [
    "text/html; charset=utf-8",
    "text/plain",
    "application/json; a=1; b=two",
    'multipart/form-data; boundary="x y"',
    'text/plain; charset="UTF-8"; name="a;b"',
    "image/svg+xml",
    "application/x-tar; mode=0644",
    "text/plain; charset=utf-8; charset=utf-8",
    "text/plain; charset=utf-8; charset=latin-1",
    "text",
    "/plain",
    "text/",
    "text/plain;",
    "text/plain; =v",
    'text/plain; a="unclosed',
    "text/plain; a=b; c=d; e=f",
    "TEXT/HTML; CHARSET=UTF-8",
    "text/plain; a=" + "x" * 100,
    "text/plain; quoted=\"a\\\"b\"",
    "application/octet-stream",
]

QP = [
    b"hello world\n",
    "café\n".encode(),
    b"=3D =20 =09\n",
    b"a soft=\r\nbreak\n",
    b"a soft=\nbreak\n",
    b"trailing=\r\n",
    b"a\r\nb\r\n",
    b"a\rb\n",
    b"=41=42=43",
    b"=zz",
    b"=4",
    b"=4z",
    b"",
    b" ",
    b"x" * 200 + b"\n",
    b"tab\there\n",
    b"line with spaces   \n",
    bytes(range(1, 32)),
]

BODIES = [
    ("xyz", b'--xyz\r\nContent-Disposition: form-data; name="a"\r\n\r\nvalue\r\n--xyz--\r\n'),
    ("xyz", b'--xyz\r\nContent-Disposition: form-data; name="a"\r\n\r\nvalue\r\n--xyz--'),
    ("b", b'preamble\r\n--b\r\nContent-Disposition: form-data; name="f"; filename="a.txt"\r\nContent-Type: text/plain\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\ncaf=C3=A9\r\n--b\r\nContent-Disposition: form-data; name="n"\r\n\r\n42\r\n--b--\r\nepilogue'),
    ("b", b'--b\nContent-Disposition: form-data; name="a"\n\nx\n--b--\n'),
    ("b", b'--b\r\n\r\nempty\r\n--b--\r\n'),
    ("b", b'--b\r\nContent-Disposition: form-data; name="a"\r\n\r\ny'),
    ("b", b'garbage without a boundary'),
    ("b", b'--b--\r\n'),
    ("b", b'--b\r\nX: y\r\nX: z\r\nContent-Disposition: form-data; name="a"\r\n\r\nv\r\n--b--\r\n'),
]


def corpus():
    rng = random.Random(SEED)
    lines = []
    media = list(MEDIA)
    for _ in range(400):
        kind = rng.choice(["text", "application", "image", "video", "x/y"])
        sub = rng.choice(["plain", "html", "json", "octet-stream", "xml"])
        params = "; ".join(f"{rng.choice(['charset', 'boundary', 'name'])}={rng.choice(['utf-8', 'x y', '\"q\"', 'A'])}"
                           for _ in range(rng.randint(0, 3)))
        media.append(f"{kind}/{sub}" + ("; " + params if params else ""))
    lines += ["P " + m.encode().hex() for m in media]
    qp = list(QP)
    for _ in range(400):
        n = rng.randint(0, 40)
        b = bytes(rng.choice(b"abc =0139\r\n\t\xe9\xff") for _ in range(n))
        qp.append(b)
    lines += ["Q " + t.hex() for t in qp]
    bodies = list(BODIES)
    for _ in range(50):
        n = rng.randint(1, 3)
        boundary = "bnd" + str(rng.randint(0, 9))
        parts = []
        for k in range(n):
            parts.append(f'--{boundary}\r\nContent-Disposition: form-data; name="p{k}"\r\n\r\nv{k}\r\n')
        bodies.append((boundary, ("preamble\r\n" + "".join(parts) + f"--{boundary}--\r\n").encode()))
    lines += ["M " + b.encode().hex() + " " + body.hex() for b, body in bodies]
    return lines


def run(exe, data):
    result = subprocess.run([str(exe)], input=data.encode(), capture_output=True, check=True,
                            cwd=ROOT, timeout=600)
    return result.stdout.decode().splitlines()


def main():
    lines = corpus()
    data = "\n".join(lines) + "\n"
    directory = ROOT / "bin/ci/mime"
    directory.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="mime-", dir=directory) as tmp:
        work = Path(tmp)
        tin = work / "mime-tin"
        subprocess.run([str(ROOT / "bin/tinc"), "-o", str(tin), "tools/ci/fixtures/mime.tin"],
                       check=True, cwd=ROOT, env={"TIN_ROOT": str(ROOT)}, timeout=300)
        go = work / "mime-go"
        subprocess.run(["go", "build", "-o", str(go), "./bench/ref/mime"], check=True, cwd=ROOT,
                       timeout=300)
        want = run(go, data)
        got = run(tin, data)
        assert len(got) == len(want), f"mime: {len(got)} lines against Go's {len(want)}"
        bad = 0
        for i, (a, b) in enumerate(zip(want, got)):
            if a != b:
                print(f"line {i + 1} (corpus line {lines[i] if i < len(lines) else '?'}):")
                print(f"  Go:  {a}")
                print(f"  Tin: {b}")
                bad += 1
                if bad >= 10:
                    break
        assert bad == 0, f"mime: {bad} of {len(want)} lines differ"
        print(f"PASS mime: {len(want)} lines ({len(lines)} corpus cases) match Go's mime, "
              f"quoted-printable and multipart")


if __name__ == "__main__":
    main()
