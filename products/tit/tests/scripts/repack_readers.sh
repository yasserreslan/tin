#!/bin/sh
# Readers while repack runs (#1007): tit log in the server's repository and fetches through tit serve run in a loop
# while pushes add packs and tit repack --all merges them again and again; no read may find a pack gone. Retired packs
# stay in packs/retired until the grace period passes. Usage: repack_readers.sh <tit> <empty directory>
set -eu
tit=$1
d=$2
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1 TIN_CORES=2
mkdir -p "$d/home"
fail() {
	echo "FAIL repack_readers: $*"
	exit 1
}
t() { "$tit" "$@"; }
port=$((20000 + ($$ + 4099) % 20000))
url="http://127.0.0.1:$port/"
t config set --user user.name Ada
t config set --user user.email ada@example.com
pub=$(t key 2>/dev/null)
mkdir "$d/server"
cd "$d/server"
t init . > /dev/null
printf 'one\n' > a.txt
t add a.txt
t commit -m first > /dev/null
echo "ada@example.com ed25519 $pub" > .tit/allowed-keys
(cd "$d/server" && exec "$tit" serve --addr "127.0.0.1:$port") > "$d/serve.log" 2>&1 &
server=$!
readers=""
trap 'kill $server $readers 2>/dev/null || true' EXIT HUP INT TERM
cd "$d"
k=0
until t clone "$url" client > /dev/null 2>&1; do
	k=$((k + 1))
	[ $k -lt 100 ] || fail "the server did not start: $(cat "$d/serve.log")"
	rm -rf client
	sleep 0.05
done
t clone "$url" reader > /dev/null
# push n: a new commit with a file of its own, one more pack on the server
push() {
	(
		cd "$d/client"
		i=0
		while [ $i -lt 40 ]; do
			echo "push $1 line $i $(date +%s%N 2>/dev/null || date +%s)"
			i=$((i + 1))
		done > "f$1.txt"
		t add "f$1.txt"
		t commit -m "push $1" > /dev/null
		t push > /dev/null
	)
}
for n in 1 2 3 4; do
	push $n
done
: > "$d/errors"
(
	while [ ! -e "$d/stop" ]; do
		t -C "$d/server" log --format=id main > /dev/null 2> "$d/log.err" || cat "$d/log.err" >> "$d/errors"
	done
) &
readers="$!"
(
	while [ ! -e "$d/stop" ]; do
		t -C "$d/reader" fetch > /dev/null 2> "$d/fetch.err" || cat "$d/fetch.err" >> "$d/errors"
	done
) &
readers="$readers $!"
for n in 5 6 7 8 9 10 11 12; do
	push $n
	t -C "$d/server" repack --all > /dev/null
done
touch "$d/stop"
wait $readers 2>/dev/null || true
readers=""
[ ! -s "$d/errors" ] || fail "a read failed while repack ran: $(head -5 "$d/errors")"
retired=$(ls "$d/server/.tit/packs/retired" 2>/dev/null | grep -c '\.pack$' || true)
[ "$retired" -gt 0 ] || fail "repack retired no pack"
[ "$(t -C "$d/server" log --format=subject -n 1)" = "push 12" ] || fail "the server's main"
echo "ok readers during repack: no read failed, $retired packs retired"
# past the grace period, the next repack deletes them
old=$(ls "$d/server/.tit/packs/retired" | grep '\.pack$')
push 13
sleep 1
TIT_RETIRED_GRACE=0 t -C "$d/server" repack --all > /dev/null
for p in $old; do
	[ ! -e "$d/server/.tit/packs/retired/$p" ] || fail "$p is still retired after the grace period"
done
t -C "$d/server" log --format=id main > /dev/null || fail "the log after the sweep"
echo "ok retired packs deleted after the grace period"
