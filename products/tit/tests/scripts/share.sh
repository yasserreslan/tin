#!/bin/sh
# Sharing over tit serve (#795, #796): clone, push, a push behind the server refused, pull and push again, fetch, an
# unsigned client refused. Usage: share.sh <tit> <empty directory>
set -eu
tit=$1
d=$2
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1 TIN_CORES=2
mkdir -p "$d/home"
fail() {
	echo "FAIL share: $*"
	exit 1
}
t() { "$tit" "$@"; }
port=$((20000 + $$ % 20000))
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
t serve --addr "127.0.0.1:$port" > "$d/serve.log" 2>&1 &
server=$!
trap 'kill $server 2>/dev/null || true' EXIT HUP INT TERM
cd "$d"
n=0
until t clone "$url" b > "$d/clone.txt" 2>&1; do
	n=$((n + 1))
	[ $n -lt 50 ] || fail "the server did not start: $(cat "$d/serve.log") $(cat "$d/clone.txt")"
	rm -rf b
	sleep 0.1
done
grep -q "^Cloned .* into b (3 objects); on main\.$" "$d/clone.txt" || fail "clone said: $(cat "$d/clone.txt")"
[ "$(cat b/a.txt)" = "one" ] || fail "the clone's file"
echo "ok clone"

t clone "$url" c > /dev/null
cd "$d/b"
printf 'two\n' >> a.txt
t commit -am "from b" > /dev/null
t push > /dev/null
[ "$(t -C "$d/server" log --format=subject -n 1)" = "from b" ] || fail "the server's main after b's push"
[ "$(t log --format=id -n 1 origin/main)" = "$(t log --format=id -n 1)" ] || fail "b's copy of the server's main"
echo "ok push"

# c is behind: its push is refused, a pull merges b's commit in, then the push goes through
cd "$d/c"
printf 'c\n' > c.txt
t add c.txt
t commit -m "from c" > /dev/null
if t push > "$d/out.txt" 2>&1; then
	fail "a push behind the server went through"
fi
grep -q "tit pull" "$d/out.txt" || fail "the refusal: $(cat "$d/out.txt")"
echo "ok a push behind the server is refused"
t pull > /dev/null
[ "$(cat a.txt)" = "one
two" ] || fail "the pull did not bring b's change"
t push > /dev/null
[ "$(t -C "$d/server" log --format=subject -n 1)" = "Merge commit 'origin/main'" ] || fail "the server's main after c's push: $(t -C "$d/server" log --format=subject -n 1)"
echo "ok pull, then push"

# b fetches c's work: its copy of the server's main moves, its own main does not
cd "$d/b"
before=$(t log --format=id -n 1)
t fetch > /dev/null
[ "$(t log --format=id -n 1)" = "$before" ] || fail "fetch moved main"
[ "$(t log --format=subject -n 1 origin/main)" = "Merge commit 'origin/main'" ] || fail "fetch did not move origin/main"
echo "ok fetch"

# undoing c's push asks first, then puts the server's main back where it was
cd "$d/c"
if t undo > "$d/out.txt" 2>&1; then
	fail "an undo of a push went through without --yes"
fi
grep -q "tit undo --yes" "$d/out.txt" || fail "the undo of a push: $(cat "$d/out.txt")"
[ "$(t -C "$d/server" log --format=subject -n 1)" = "Merge commit 'origin/main'" ] || fail "a refused undo moved the server"
t undo --yes > /dev/null
[ "$(t -C "$d/server" log --format=subject -n 1)" = "from b" ] || fail "the server's main after undoing the push: $(t -C "$d/server" log --format=subject -n 1)"
[ "$(t log --format=subject -n 1 origin/main)" = "from b" ] || fail "c's origin/main after undoing the push"
t push > /dev/null
[ "$(t -C "$d/server" log --format=subject -n 1)" = "Merge commit 'origin/main'" ] || fail "the push again"
echo "ok undoing a push puts the server's branch back, after asking"

# a client with no key is refused
mkdir "$d/stranger"
if HOME="$d/stranger" XDG_CONFIG_HOME="$d/stranger" t clone "$url" "$d/s" > "$d/out.txt" 2>&1; then
	fail "an unsigned clone of a private repository"
fi
grep -q "Unauthorized" "$d/out.txt" || fail "the stranger's refusal: $(cat "$d/out.txt")"
echo "ok an unsigned client is refused"
