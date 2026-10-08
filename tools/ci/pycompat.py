"""Helpers the checks still written in Python share while they are ported to Tin (tools/ci/nettest is their Tin successor).
Deleted with the last of them."""
import base64
import hashlib
import os
import random
import socket
import struct
import time

PORT = 0


def free_port():
    """A free port below the ephemeral ranges (Linux 32768+, macOS 49152+)."""
    for _ in range(500):
        port = random.randint(20000, 30000)
        with socket.socket() as s:
            try:
                s.bind(('127.0.0.1', port))
            except OSError:
                continue
            return port
    raise RuntimeError('no free port below the ephemeral range')


def connect(path='/echo'):
    s = socket.create_connection(('127.0.0.1', PORT), timeout=5)
    key = base64.b64encode(os.urandom(16)).decode()
    s.sendall(('GET %s HTTP/1.1\r\nHost: x\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n' % (path, key)).encode())
    buf = b''
    while b'\r\n\r\n' not in buf:
        buf += s.recv(4096)
    head, rest = buf.split(b'\r\n\r\n', 1)
    acc = base64.b64encode(hashlib.sha1((key + '258EAFA5-E914-47DA-95CA-C5AB0DC85B11').encode()).digest()).decode()
    assert head.startswith(b'HTTP/1.1 101'), head
    assert acc.encode() in head, head
    return [s, rest]


def send(c, op, data, fin=True, mask=True):
    b0 = (0x80 if fin else 0) | op
    n = len(data)
    if n < 126:
        h = struct.pack('BB', b0, (0x80 if mask else 0) | n)
    elif n < 65536:
        h = struct.pack('>BBH', b0, (0x80 if mask else 0) | 126, n)
    else:
        h = struct.pack('>BBQ', b0, (0x80 if mask else 0) | 127, n)
    if mask:
        k = os.urandom(4)
        data = bytes(x ^ k[i % 4] for i, x in enumerate(data))
        h += k
    c[0].sendall(h + data)


def recvn(c, n):
    while len(c[1]) < n:
        d = c[0].recv(65536)
        if not d:
            raise EOFError
        c[1] += d
    out, c[1] = c[1][:n], c[1][n:]
    return out


def recv(c):
    b0, b1 = recvn(c, 2)
    n = b1 & 127
    if n == 126:
        n = struct.unpack('>H', recvn(c, 2))[0]
    elif n == 127:
        n = struct.unpack('>Q', recvn(c, 8))[0]
    assert not b1 & 0x80
    return b0 & 15, recvn(c, n)


def request(port, path, timeout=3):
    s = socket.create_connection(('127.0.0.1', port), timeout=timeout)
    s.sendall(('GET %s HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n' % path).encode())
    return s


def response(s):
    with s:
        data = b''
        while True:
            part = s.recv(65536)
            if not part:
                break
            data += part
    head, body = data.split(b'\r\n\r\n', 1)
    return int(head.split()[1]), body


def eventually(check, seconds=3):
    end = time.monotonic() + seconds
    while time.monotonic() < end:
        if check():
            return
        time.sleep(.01)
    assert check(), 'condition did not become true'


def server_ready(port, server):
    assert server.poll() is None, 'server crashed during startup'
    try:
        with socket.create_connection(('127.0.0.1', port), timeout=.1):
            return True
    except OSError:
        return False
