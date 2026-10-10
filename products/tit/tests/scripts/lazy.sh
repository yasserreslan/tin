#!/bin/sh
# Lazy clones (#803): tit clone --lazy takes every commit and tree and only the files it checks out; tit log then
# fetches nothing; reading an old version (show, a switch) fetches what it needs and keeps it; with the server gone
# the history still reads. Usage: lazy.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1
fail() {
	echo "FAIL lazy: $*"
	exit 1
}
t() { "$tit" "$@"; }
count() {
	find "$1/.tit/objects" "$1/.tit/packs" -type f 2> /dev/null | grep -v '/tmp-' | wc -l | tr -d ' '
}
mkdir -p "$HOME"
t config set --user user.name Ada
t config set --user user.email ada@example.com
mkdir "$d/server"
cd "$d/server"
t init . > /dev/null
i=1
while [ $i -le 20 ]; do
	printf 'version %s of a\n' $i > a.txt
	printf 'version %s of b\n' $i > "b$i.txt"
	t add . > /dev/null
	t commit -m "commit $i" > /dev/null
	i=$((i + 1))
done
port=$((20000 + $$ % 20000))
"$tit" serve --public --addr "127.0.0.1:$port" > "$d/serve.log" 2>&1 &
server=$!
trap 'kill $server 2> /dev/null || true' EXIT HUP INT TERM
cd "$d"
n=0
until t clone --lazy "http://127.0.0.1:$port/" lazy > "$d/clone.txt" 2>&1; do
	n=$((n + 1))
	[ $n -lt 50 ] || fail "the server did not start: $(cat "$d/serve.log") $(cat "$d/clone.txt")"
	rm -rf lazy
	sleep 0.1
done
grep -q "file contents as they are read" "$d/clone.txt" || fail "clone said: $(cat "$d/clone.txt")"
[ "$(cat lazy/a.txt)" = "version 20 of a" ] || fail "the lazy clone's files"
[ "$(cd lazy && t status -s)" = "" ] || fail "status in the lazy clone: $(cd lazy && t status -s)"
echo "ok a lazy clone checks out the last commit"

cd "$d/lazy"
before=$(count .)
t log > /dev/null
t log --oneline > /dev/null
[ "$(t log --format=subject | wc -l | tr -d ' ')" = 20 ] || fail "the lazy clone's history"
[ "$(count .)" = "$before" ] || fail "tit log fetched something: $before objects, then $(count .)"
echo "ok tit log in a lazy clone fetches no file contents"

t show HEAD~10 | grep -q '^+version 10 of a$' || fail "show of an old commit: $(t show HEAD~10 | tail -3)"
after=$(count .)
[ "$after" -gt "$before" ] || fail "show fetched nothing"
t show HEAD~10 > /dev/null
[ "$(count .)" = "$after" ] || fail "the second show fetched again"
echo "ok an old file is fetched on first read, once"

t switch --detach HEAD~15 > /dev/null
[ "$(cat a.txt)" = "version 5 of a" ] || fail "the switch: $(cat a.txt)"
before=$(count .)
[ "$(t status -s)" = "" ] || fail "status on the old commit: $(t status -s)"
[ "$(count .)" = "$before" ] || fail "status on a detached HEAD fetched something: $before objects, then $(count .)"
short=$(t log -n 1 --format=id | cut -c1-12)
t show "$short" > /dev/null || fail "show $short on a detached HEAD"
t switch main > /dev/null
[ "$(cat a.txt)" = "version 20 of a" ] || fail "back on main"
echo "ok a switch in a lazy clone fetches the files it writes"

# a switch that writes many files keeps them as the one pack it fetched, not as a loose file each
cd "$d/server"
mkdir many
i=1
while [ $i -le 100 ]; do
	printf 'file %s\n' $i > "many/f$i.txt"
	i=$((i + 1))
done
t add . > /dev/null
t commit -m "many files" > /dev/null
cd "$d/lazy"
loose=$(find .tit/objects -type f | wc -l | tr -d ' ')
t pull > "$d/pull.txt" 2>&1 || fail "pull in the lazy clone: $(cat "$d/pull.txt")"
[ "$(cat many/f73.txt)" = "file 73" ] || fail "the pulled files: $(ls many | wc -l)"
[ "$(find .tit/objects -type f | wc -l | tr -d ' ')" -lt $((loose + 10)) ] || fail "the pulled files were kept loose: $loose loose objects, then $(find .tit/objects -type f | wc -l)"
[ "$(t status -s)" = "" ] || fail "status after the pull: $(t status -s)"
echo "ok a switch that writes many files keeps the pack it fetched"

kill $server
wait $server 2> /dev/null || true
t log > /dev/null || fail "log with the server gone"
if t show HEAD~3 > "$d/out.txt" 2>&1; then
	fail "an unfetched file read with the server gone"
fi
grep -q "lazy clone reads file contents from" "$d/out.txt" || fail "the error with the server gone: $(cat "$d/out.txt")"
echo "ok with the server gone, log works and a missing file says where it comes from"
