#!/bin/sh
# A clone streams a large file from its pack (#1076): tit serve sends the pack's entries from the file a window at a
# time, so cloning a 40 MiB file that does not compress costs the server a few MiB, not the file. The peak is read from
# /proc (Linux); elsewhere only the clone is checked. Usage: fetch_memory.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1
fail() {
	echo "FAIL fetch_memory: $*"
	exit 1
}
t() { "$tit" "$@"; }
mkdir -p "$HOME" "$d/server"
t config set --user user.name Ada
t config set --user user.email ada@example.com
pub=$(t key 2> /dev/null)
cd "$d/server"
t init . > /dev/null
printf 'small\n' > small.txt
t add . > /dev/null
t commit -m one > /dev/null
echo "ada@example.com ed25519 $pub" > .tit/allowed-keys
port=$((20000 + ($$ + 11) % 20000))
url="http://127.0.0.1:$port/"
server=
trap '[ -n "$server" ] && kill $server 2> /dev/null || true' EXIT HUP INT TERM
serve() {
	(cd "$d/server" && TIN_CORES=1 exec "$tit" serve --addr "127.0.0.1:$port") > "$d/serve.log" 2>&1 &
	server=$!
}
serve
cd "$d"
n=0
until t clone "$url" a > /dev/null 2>&1; do
	n=$((n + 1))
	[ $n -lt 50 ] || fail "the server did not start: $(cat "$d/serve.log")"
	rm -rf a
	sleep 0.1
done
# the large file arrives in a push, so the server holds it in a pack
cd a
head -c 41943040 /dev/urandom > big.bin
t add big.bin > /dev/null
t commit -m big > /dev/null
t push > /dev/null
cd "$d"
kill $server
wait $server 2> /dev/null || true
serve
n=0
until t clone "$url" b > "$d/clone.txt" 2>&1; do
	n=$((n + 1))
	[ $n -lt 50 ] || fail "the clone: $(cat "$d/clone.txt") $(cat "$d/serve.log")"
	rm -rf b
	sleep 0.1
done
cmp a/big.bin b/big.bin || fail "the cloned file differs"
echo "ok a clone of a 40 MiB file"
if [ -r "/proc/$server/status" ]; then
	peak=$(sed -n 's/^VmHWM:[^0-9]*\([0-9]*\) kB$/\1/p' "/proc/$server/status")
	[ "$peak" -lt 32768 ] || fail "the server peaked at $peak KiB for one clone of a 40 MiB file"
	echo "ok the server's peak stays far under the file's size"
fi
