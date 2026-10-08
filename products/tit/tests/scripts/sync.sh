#!/bin/sh
# tit sync (#797) against tit serve: a stack is pushed; when the trunk moves on, sync rebases the stack onto it
# (same change ids), moves the local trunk and replaces the server's branch; a rebase that records a conflict is not
# pushed until the conflict is resolved. Usage: sync.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1 TIN_CORES=2
mkdir -p "$d/home"
fail() {
	echo "FAIL sync: $*"
	exit 1
}
t() { "$tit" "$@"; }
port=$((20000 + ($$ + 7) % 20000))
url="http://127.0.0.1:$port/"
t config set --user user.name Ada
t config set --user user.email ada@example.com
pub=$(t key 2>/dev/null)
mkdir "$d/server"
cd "$d/server"
t init . > /dev/null
printf 'one\ntwo\nthree\n' > a.txt
printf 'b\n' > b.txt
t add a.txt b.txt
t commit -m base > /dev/null
echo "ada@example.com ed25519 $pub" > .tit/allowed-keys
t serve --addr "127.0.0.1:$port" > "$d/serve.log" 2>&1 &
server=$!
trap 'kill $server 2>/dev/null || true' EXIT HUP INT TERM
cd "$d"
n=0
until t clone "$url" b > /dev/null 2>&1; do
	n=$((n + 1))
	[ $n -lt 50 ] || fail "the server did not start: $(cat "$d/serve.log")"
	rm -rf b
	sleep 0.1
done
t clone "$url" c > /dev/null

cd "$d/b"
t switch -c feat > /dev/null
printf 'b, from feat\n' > b.txt
t commit -am "feat one" > /dev/null
printf 'new\n' > new.txt
t add new.txt
t commit -m "feat two" > /dev/null
changes=$(t log --format=change -n 2 | tr '\n' ' ')
t sync > /dev/null
[ "$(t -C "$d/server" log --format=subject -n 1 feat)" = "feat two" ] || fail "the first sync did not push feat"
echo "ok sync pushes a new stack"

cd "$d/c"
printf 'one\ntwo\nthree\nfour, from c\n' > a.txt
t commit -am "c on main" > /dev/null
t push > /dev/null
cd "$d/b"
t sync > "$d/out.txt" || fail "sync: $(cat "$d/out.txt")"
grep -q "rebased 2 changes" "$d/out.txt" || fail "sync said: $(cat "$d/out.txt")"
[ "$(t log --format=subject -n 3 | tr '\n' '|')" = "feat two|feat one|c on main|" ] || fail "the local stack: $(t log --format=subject -n 3)"
[ "$(t log --format=change -n 2 | tr '\n' ' ')" = "$changes" ] || fail "the rebase changed the change ids"
[ "$(t log --format=subject -n 1 main)" = "c on main" ] || fail "main did not follow origin/main"
[ "$(t -C "$d/server" log --format=subject -n 3 feat | tr '\n' '|')" = "feat two|feat one|c on main|" ] || fail "the server's feat: $(t -C "$d/server" log --format=subject -n 3 feat)"
grep -q 'four, from c' a.txt || fail "the working directory did not follow"
echo "ok sync rebases onto the new trunk and replaces the server's branch"

# c changes the line feat one changed: the rebase records a conflict, and nothing is pushed until it is resolved
cd "$d/c"
printf 'b, from c\n' > b.txt
t commit -am "c changes b" > /dev/null
t push > /dev/null
cd "$d/b"
before=$(t -C "$d/server" log --format=id -n 1 feat)
if t sync > "$d/out.txt" 2>&1; then
	fail "a stack with a conflict was pushed"
fi
grep -q "records conflicts" "$d/out.txt" || fail "the refusal: $(cat "$d/out.txt")"
[ "$(t -C "$d/server" log --format=id -n 1 feat)" = "$before" ] || fail "the server's feat moved"
one=$(t log --format=change -n 1 HEAD~1)
t edit "$one" > /dev/null
printf 'b, from c and feat\n' > b.txt
t add b.txt
t commit --amend > /dev/null
t sync > /dev/null || fail "sync after resolving"
[ "$(t -C "$d/server" log --format=subject -n 3 feat | tr '\n' '|')" = "feat two|feat one|c changes b|" ] || fail "after resolving: $(t -C "$d/server" log --format=subject -n 3 feat)"
echo "ok a conflict waits for tit edit, then sync pushes"

# two users with stacks of their own: the trunk moves, and each one's sync rebases and pushes their stack
cd "$d/c"
t pull > /dev/null
t switch -c cfeat > /dev/null
printf 'c one\n' > c1.txt
t add c1.txt
t commit -m "cfeat one" > /dev/null
printf 'c two\n' > c2.txt
t add c2.txt
t commit -m "cfeat two" > /dev/null
cchanges=$(t log --format=change -n 2 | tr '\n' ' ')
t sync > /dev/null || fail "c's first sync"
cd "$d/b"
t switch main > /dev/null
t pull > /dev/null
printf 'one\ntwo\nthree\nfour, from c\nfive, from b on main\n' > a.txt
t commit -am "b on main" > /dev/null
t push > /dev/null
t switch feat > /dev/null
bchanges=$(t log --format=change -n 2 | tr '\n' ' ')
t sync > /dev/null || fail "b's sync after main moved"
cd "$d/c"
t sync > /dev/null || fail "c's sync after main moved"
trunk=$(t -C "$d/server" log --format=id -n 1 main)
[ "$(t -C "$d/server" log --format=id -n 1 feat~2)" = "$trunk" ] || fail "the server's feat is not on the new main"
[ "$(t -C "$d/server" log --format=id -n 1 cfeat~2)" = "$trunk" ] || fail "the server's cfeat is not on the new main"
[ "$(t -C "$d/server" log --format=change -n 2 feat | tr '\n' ' ')" = "$bchanges" ] || fail "feat's change ids"
[ "$(t -C "$d/server" log --format=change -n 2 cfeat | tr '\n' ' ')" = "$cchanges" ] || fail "cfeat's change ids"
echo "ok two users' stacks both rebased onto the moved trunk and pushed"
