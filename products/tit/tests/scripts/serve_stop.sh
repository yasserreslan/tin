#!/bin/sh
# tit serve stops cleanly (#1055): after serving a clone, SIGTERM and SIGINT each end it with status 0 and nothing but
# its first line in its log (no panic). Usage: serve_stop.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1 TIN_CORES=2
mkdir -p "$d/home"
fail() {
	echo "FAIL serve_stop: $*"
	exit 1
}
t() { "$tit" "$@"; }
t config set --user user.name Ada
t config set --user user.email ada@example.com
mkdir "$d/server"
cd "$d/server"
t init . > /dev/null
printf 'one\n' > a.txt
t add a.txt
t commit -m first > /dev/null
port=$((20000 + ($$ + 1777) % 20000))
for sig in TERM INT; do
	(cd "$d/server" && exec "$tit" serve --public --addr "127.0.0.1:$port") > "$d/serve.log" 2>&1 &
	server=$!
	rm -rf "$d/c"
	n=0
	until t clone "http://127.0.0.1:$port/" "$d/c" > /dev/null 2>&1; do
		n=$((n + 1))
		[ $n -lt 50 ] || fail "the server did not start: $(cat "$d/serve.log")"
		rm -rf "$d/c"
		sleep 0.1
	done
	kill -$sig $server
	set +e
	wait $server
	rc=$?
	set -e
	[ $rc -eq 0 ] || fail "SIG$sig: tit serve ended with status $rc: $(cat "$d/serve.log")"
	[ "$(wc -l < "$d/serve.log")" -eq 1 ] || fail "SIG$sig: the log: $(cat "$d/serve.log")"
	echo "ok SIG$sig stops tit serve cleanly"
done
