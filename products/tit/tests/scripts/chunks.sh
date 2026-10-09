#!/bin/sh
# Large files in chunks (#803): a 64 MiB file is stored in content-defined chunks; overwriting 1 MiB in its middle
# adds about 1 MiB to the store (the criterion: under 5 MiB, for a 2 GiB file on Linux; TIT_CHUNKS_MB sets the size
# here), inserting bytes near its start shifts nothing else; the old version checks out byte for byte, written a
# chunk at a time, and status is clean. Usage: chunks.sh <tit> <dir>
set -eu
tit=$1
d=$2
mb=${TIT_CHUNKS_MB:-64}
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1
fail() {
	echo "FAIL chunks: $*"
	exit 1
}
t() { "$tit" "$@"; }
kib() {
	du -sk "$1/.tit/objects" | cut -f1
}
mkdir -p "$HOME" "$d/r"
cd "$d/r"
t init . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
# incompressible data from a fixed seed (perl's rand, seeded, is the same on every platform), so the
# chunk cut points are the same on every run: the growth figures below are then exact, not a range
# (#945: with /dev/urandom one run in six measured a cascade that broke the criterion)
perl -e 'srand(945); my $n = '$mb' * 1048576; my $b = "C" x 65536;
	binmode STDOUT;
	while ($n > 0) { my $k = $n < 65536 ? $n : 65536; print pack("C$k", map { int(rand(256)) } 1 .. $k); $n -= $k }' > big.bin
cp big.bin "$d/v1.bin"
t add big.bin
t commit -m "a large file" > /dev/null
[ "$(ls .tit/objects/chunks | wc -l)" -gt 0 ] || fail "no chunks"
k1=$(kib .)
perl -e 'open F, "+<", "big.bin" or die; seek F, '$((mb * 524288))', 0; print F "x" x 1048576; close F'
t commit -am "1 MiB overwritten in the middle" > /dev/null
k2=$(kib .)
[ $((k2 - k1)) -lt 5120 ] || fail "1 MiB overwritten added $((k2 - k1)) KiB"
echo "ok overwriting 1 MiB in the middle of $mb MiB added $((k2 - k1)) KiB"
{ printf 'a few bytes at the start'; cat big.bin; } > x && mv x big.bin
t commit -am "bytes inserted near the start" > /dev/null
k3=$(kib .)
[ $((k3 - k2)) -lt 5120 ] || fail "an insertion near the start added $((k3 - k2)) KiB"
echo "ok inserting bytes near the start added $((k3 - k2)) KiB: the cuts after it do not move"
[ "$(t status -s)" = "" ] || fail "status: $(t status -s)"
echo "ok status is clean"
# a checkout writes the file a chunk at a time
t switch --detach HEAD~2 > /dev/null
cmp -s big.bin "$d/v1.bin" || fail "the first version does not check out byte for byte"
t switch main > /dev/null
echo "ok the first version checks out byte for byte, a chunk at a time"
