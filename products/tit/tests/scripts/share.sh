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

# ship: a tag with the changelog since the last tag, signed, then the branch and the tag pushed
cd "$d/c"
printf 'three\n' >> a.txt
t commit -am "Add three" > /dev/null
printf 'four\n' >> a.txt
t commit -am "Add four" > /dev/null
t ship v1.0 --dry-run > "$d/out.txt"
t tag | grep -q v1.0 && fail "a dry run made the tag"
grep -q "^- Add four (" "$d/out.txt" || fail "the changelog: $(cat "$d/out.txt")"
printf 'dirty\n' >> a.txt
t ship v1.0 > "$d/out.txt" 2>&1 && fail "ship with uncommitted changes"
t commit -am "Add dirty" > /dev/null
t ship v1.0 > "$d/out.txt"
grep -q "^Pushed the tag v1.0 to origin\.$" "$d/out.txt" || fail "ship said: $(cat "$d/out.txt")"
[ "$(t -C "$d/server" tag)" = "v1.0" ] || fail "the server's tags: $(t -C "$d/server" tag)"
[ "$(t -C "$d/server" log --format=subject -n 1)" = "Add dirty" ] || fail "ship did not push the branch"
[ "$(t -C "$d/server" log --format=id -n 1 v1.0)" = "$(t log --format=id -n 1)" ] || fail "the server's v1.0 is not the shipped commit"
printf 'five\n' >> a.txt
t commit -am "Add five" > /dev/null
t ship v1.1 > "$d/out.txt"
grep -q "^Changes since v1.0 (1):$" "$d/out.txt" || fail "the second changelog: $(cat "$d/out.txt")"
grep -q "^- Add five (" "$d/out.txt" || fail "the second changelog: $(cat "$d/out.txt")"
[ "$(t -C "$d/server" tag | tr '\n' ' ')" = "v1.0 v1.1 " ] || fail "the server's tags: $(t -C "$d/server" tag)"
t ship v1.1 > "$d/out.txt" 2>&1 && fail "a tag shipped twice"
echo "ok ship"
# a clone takes the annotated tags along with the commits they tag
t clone "$url" "$d/tagged" > /dev/null || fail "a clone of a repository with annotated tags"
[ "$(t -C "$d/tagged" tag | tr '\n' ' ')" = "v1.0 v1.1 " ] || fail "the clone's tags: $(t -C "$d/tagged" tag)"
[ "$(t -C "$d/tagged" log --format=subject -n 1 v1.0)" = "Add dirty" ] || fail "the clone's v1.0"
echo "ok a clone takes the annotated tags"

# a client with no key is refused
mkdir "$d/stranger"
if HOME="$d/stranger" XDG_CONFIG_HOME="$d/stranger" t clone "$url" "$d/s" > "$d/out.txt" 2>&1; then
	fail "an unsigned clone of a private repository"
fi
grep -q "Unauthorized" "$d/out.txt" || fail "the stranger's refusal: $(cat "$d/out.txt")"
echo "ok an unsigned client is refused"
