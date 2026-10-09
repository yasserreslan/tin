#!/bin/sh
# tit focus (#803): only the chosen directories (and the root's files) are checked out; status and diff are clean,
# commit -a and add . keep the files outside the focus as they were, a switch writes only inside the focus, a file
# leaving the focus with changes refuses, --all brings everything back; with a lazy clone, the files outside the
# focus are never fetched. Usage: focus.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1
fail() {
	echo "FAIL focus: $*"
	exit 1
}
t() { "$tit" "$@"; }
mkdir -p "$HOME" "$d/r"
t config set --user user.name Ada
t config set --user user.email ada@example.com
cd "$d/r"
t init . > /dev/null
mkdir -p app/src lib docs
printf 'app\n' > app/src/main.txt
printf 'lib\n' > lib/lib.txt
printf 'docs\n' > docs/readme.txt
printf 'root\n' > top.txt
t add .
t commit -m base > /dev/null
t focus app > "$d/out.txt"
grep -q "^Focused on app: 0 files written, 2 removed\.$" "$d/out.txt" || fail "focus said: $(cat "$d/out.txt")"
[ -e app/src/main.txt ] && [ -e top.txt ] || fail "app or the root's files are gone"
[ ! -e lib/lib.txt ] && [ ! -e docs ] && [ ! -e lib ] || fail "lib or docs are still here"
[ "$(t status -s)" = "" ] || fail "status in focus: $(t status -s)"
[ "$(t diff)" = "" ] || fail "diff in focus"
[ "$(t focus)" = "app" ] || fail "the focus: $(t focus)"
echo "ok only app and the root's files are checked out, and status is clean"

printf 'app, changed\n' > app/src/main.txt
t add . > /dev/null
t commit -am "change app" > /dev/null
[ "$(t diff HEAD~1 HEAD --stat | grep -c 'lib\|docs' || true)" = 0 ] || fail "a commit in focus changed lib or docs: $(t diff HEAD~1 HEAD --stat)"
t focus --all > /dev/null
[ "$(cat lib/lib.txt)" = lib ] && [ "$(cat docs/readme.txt)" = docs ] || fail "the files outside the focus changed in the commits"
echo "ok commit -a and add . in focus keep the other files as committed"

t focus lib > /dev/null
t switch -c side > /dev/null
t focus --all > /dev/null
printf 'docs, on side\n' > docs/readme.txt
printf 'lib, on side\n' > lib/lib.txt
t commit -am "side" > /dev/null
t switch main > /dev/null
t focus lib > /dev/null
t switch side > /dev/null
[ "$(cat lib/lib.txt)" = "lib, on side" ] || fail "the switch did not write lib"
[ ! -e docs ] || fail "the switch wrote docs outside the focus"
[ "$(t status -s)" = "" ] || fail "status after the switch: $(t status -s)"
echo "ok a switch writes only inside the focus"

printf 'unsaved\n' >> lib/lib.txt
if t focus app > "$d/out.txt" 2>&1; then
	fail "focus left a changed file"
fi
grep -q "lib/lib.txt" "$d/out.txt" || fail "the refusal: $(cat "$d/out.txt")"
grep -q unsaved lib/lib.txt || fail "the refused focus touched lib"
echo "ok a changed file leaving the focus refuses"
t commit -am "lib unsaved" > /dev/null
t focus --all > /dev/null
[ "$(cat docs/readme.txt)" = "docs, on side" ] || fail "--all did not bring docs back"
[ "$(t status -s)" = "" ] || fail "status after --all: $(t status -s)"
echo "ok focus --all brings everything back"

# a lazy clone in focus never fetches the files outside it
port=$((20000 + $$ % 20000))
"$tit" serve --public --addr "127.0.0.1:$port" > "$d/serve.log" 2>&1 &
server=$!
trap 'kill $server 2> /dev/null || true' EXIT HUP INT TERM
cd "$d"
n=0
until t clone --lazy "http://127.0.0.1:$port/" lazy > /dev/null 2>&1; do
	n=$((n + 1))
	[ $n -lt 50 ] || fail "the server did not start"
	rm -rf lazy
	sleep 0.1
done
cd lazy
t focus app > /dev/null
# side changes only lib and docs, both outside the focus: the switch needs nothing from the server
kill $server
wait $server 2> /dev/null || true
t switch -c side origin/side > /dev/null || fail "a switch in focus needed the server"
t log > /dev/null
[ "$(t status -s)" = "" ] || fail "status in a lazy clone in focus: $(t status -s)"
[ "$(cat lib/lib.txt 2> /dev/null || true)" = "" ] || fail "lib was written outside the focus"
echo "ok a lazy clone in focus switches, with the server gone, without the files outside it"
