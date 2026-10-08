# bzip2 streams for the bzip2 twin

These streams are inputs for `tools/ci/bzip2_check.tin` (#754), which compares `squash.Bunzip2` with
Go's `compress/bzip2`. Go's package only decodes, so they were made once with Python's `bz2` module (libbzip2,
the reference encoder) and checked in; nothing runs Python in CI. To make them again:

```python
import bz2, random, sys, os
out = sys.argv[1]
r = random.Random(754)
def put(name, data, level):
    with open(os.path.join(out, name), "wb") as f:
        f.write(bz2.compress(data, level))
vocab = ["".join(r.choice("abcdefghijklmnopqrstuvwxyz") for _ in range(r.randint(2, 9))) for _ in range(300)]
def text(n):
    words = []
    k = 0
    while k < n:
        w = r.choice(vocab)
        words.append(w)
        k += len(w) + 1
    return " ".join(words).encode()[:n]
def fib(n):
    a, b = b"a", b"ab"
    while len(b) < n:
        a, b = b, b + a
    return b[:n]
put("empty.bz2", b"", 9)
put("byte.bz2", b"a", 1)
put("hello.bz2", b"hello, world\n", 9)
put("all256.bz2", bytes(range(256)) * 4, 3)
put("runs.bz2", b"".join(bytes([i % 256]) * i for i in range(1, 300)) + b"".join(b"x" * k + b"y" for k in range(1, 12)), 2)
put("random.bz2", r.randbytes(3000), 9)
put("unicode.bz2", ("héllo wörld, مرحبا بالعالم, 你好世界, 🙂🚀\n" * 200).encode(), 4)
for level in range(1, 10):
    n = 110000 if level in (1, 9) else 20000
    put("text%d.bz2" % level, text(n), level)
put("fib9.bz2", fib(1000000), 9)
put("fib1.bz2", fib(250000), 1)
```

Run it with the directory to write to (`python3 -I gen.py toolchain/tests/data/bzip2`). The check
derives its other cases (streams back to back, cut and flipped streams, lowered block sizes) from
these and from Go's own `src/compress/bzip2/testdata`.
