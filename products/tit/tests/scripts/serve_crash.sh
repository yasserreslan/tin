#!/bin/sh
# A server killed at every crash point of a push (#1006, design/tit.md §16): tit serve runs with TIT_CRASH_AT=n for
# every n its push reaches; after each crash the next tit command in the server's repository recovers, the refs are
# exactly as before the push or as after it, and every branch reads (no ref names a missing object). A crash leaves at
# most a pending pack in packs/staged, which nothing reaches. Usage: serve_crash.sh <tit> <empty directory>
set -eu
tit=$1
d=$2
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1 TIN_CORES=1
mkdir -p "$d/home"
fail() {
	echo "FAIL serve_crash: $*"
	exit 1
}
t() { "$tit" "$@"; }
port=$((20000 + ($$ + 7919) % 20000))
url="http://127.0.0.1:$port/"
server=0
trap '[ $server -eq 0 ] || kill $server 2>/dev/null || true' EXIT HUP INT TERM
# start <dir> <n>: tit serve on dir, crashing at its n-th crash point (0: never), once it answers a fetch
start() {
	(cd "$1" && TIT_CRASH_AT=$2 exec "$tit" serve --addr "127.0.0.1:$port") > "$d/serve.log" 2>&1 &
	server=$!
	k=0
	until t -C "$d/probe" fetch > /dev/null 2>&1; do
		k=$((k + 1))
		[ $k -lt 100 ] || fail "the server did not start: $(cat "$d/serve.log")"
		sleep 0.05
	done
}
snap() {
	(
		cd "$1"
		find .tit/refs -type f | sort | while read f; do echo "$f $(cat "$f")"; done
	)
}
readable() {
	(
		cd "$1"
		for b in $(find .tit/refs/heads -type f | sed 's|.tit/refs/heads/||'); do
			t log --format=id "$b" > /dev/null || exit 1
		done
	)
}

t config set --user user.name Ada
t config set --user user.email ada@example.com
pub=$(t key 2>/dev/null)
mkdir "$d/tpl"
cd "$d/tpl"
t init . > /dev/null
printf 'one\n' > a.txt
t add a.txt
t commit -m first > /dev/null
echo "ada@example.com ed25519 $pub" > .tit/allowed-keys
cp -R "$d/tpl" "$d/w"
(cd "$d/w" && exec "$tit" serve --addr "127.0.0.1:$port") > "$d/serve.log" 2>&1 &
server=$!
cd "$d"
k=0
until t clone "$url" probe > /dev/null 2>&1; do
	k=$((k + 1))
	[ $k -lt 100 ] || fail "the server did not start: $(cat "$d/serve.log")"
	rm -rf probe
	sleep 0.05
done
t clone "$url" client > /dev/null
kill $server
wait $server 2>/dev/null || true
server=0
cd "$d/client"
printf 'two\n' >> a.txt
printf 'b\n' > b.txt
t add b.txt
t commit -am second > /dev/null
before=$(snap "$d/tpl")
# the push without a crash: what "after" is
rm -rf "$d/w" "$d/c"
cp -R "$d/tpl" "$d/w"
cp -R "$d/client" "$d/c"
start "$d/w" 0
(cd "$d/c" && t push > /dev/null 2>&1) || fail "the push does not go through without a crash"
kill $server
wait $server 2>/dev/null || true
server=0
after=$(snap "$d/w")
[ "$before" != "$after" ] || fail "the push moved no ref"
n=1
while :; do
	rm -rf "$d/w" "$d/c"
	cp -R "$d/tpl" "$d/w"
	cp -R "$d/client" "$d/c"
	start "$d/w" $n
	(cd "$d/c" && t push > /dev/null 2>&1) || true
	if kill -0 $server 2>/dev/null; then
		# the push reached no n-th crash point: every point is covered
		kill $server
		wait $server 2>/dev/null || true
		server=0
		[ "$(snap "$d/w")" = "$after" ] || fail "the last push did not go through"
		break
	fi
	set +e
	wait $server
	rc=$?
	set -e
	server=0
	[ $rc -eq 86 ] || fail "crash $n: the server ended with status $rc: $(cat "$d/serve.log")"
	(cd "$d/w" && t status -s > /dev/null 2>&1) || fail "crash $n: the next command fails"
	now=$(snap "$d/w")
	[ "$now" = "$before" ] || [ "$now" = "$after" ] || fail "crash $n: the server's refs are neither before nor after:
$now"
	readable "$d/w" || fail "crash $n: a branch does not read"
	[ ! -e "$d/w/.tit/oplog/pending.op" ] || fail "crash $n: an operation is still pending"
	n=$((n + 1))
done
[ $n -gt 2 ] || fail "only $((n - 1)) crash points in a push"
echo "ok serve crash: $((n - 1)) crash points in a push, each before or after"
