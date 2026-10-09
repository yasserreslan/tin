#!/bin/sh
# tit prune and tit purge (#1010): a deleted branch's objects stay while the oplog may undo its deletion and go once
# the retention window is over; purge refuses an object a ref reaches, removes an unreachable one from every pack at
# once and records it; undo inside the window still works after a prune; loose objects go too; a crash at any point
# of a prune leaves every branch readable. Usage: prune.sh <tit> <empty directory>
set -eu
tit=$1
d=$2
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1 TIN_CORES=2
mkdir -p "$d/home"
fail() {
	echo "FAIL prune: $*"
	exit 1
}
t() { "$tit" "$@"; }
port=$((20000 + ($$ + 6151) % 20000))
url="http://127.0.0.1:$port/"
t config set --user user.name Ada
t config set --user user.email ada@example.com
pub=$(t key 2>/dev/null)
blobOf() { # blobOf <repo> <commit> <path>
	tr=$(t -C "$1" cat "$2" | head -1 | awk '{print $2}')
	t -C "$1" cat "$tr" | awk -v p="$3" '$4 == p {print $3}'
}

# a server whose objects arrive in packs: main, and a branch with a secret pushed by mistake
mkdir "$d/s"
cd "$d/s"
t init . > /dev/null
printf 'one\n' > a.txt
t add a.txt
t commit -m first > /dev/null
echo "ada@example.com ed25519 $pub" > .tit/allowed-keys
(cd "$d/s" && exec "$tit" serve --addr "127.0.0.1:$port") > "$d/serve.log" 2>&1 &
server=$!
trap 'kill $server 2>/dev/null || true' EXIT HUP INT TERM
cd "$d"
k=0
until t clone "$url" c > /dev/null 2>&1; do
	k=$((k + 1))
	[ $k -lt 100 ] || fail "the server did not start: $(cat "$d/serve.log")"
	rm -rf c
	sleep 0.05
done
cd "$d/c"
printf 'two\n' >> a.txt
t commit -am second > /dev/null
t push > /dev/null
t branch side > /dev/null
t switch side > /dev/null
printf 'SECRET-%s\n' "$$" > secret.txt
t add secret.txt
t commit -m "oops" > /dev/null
t push > /dev/null
t switch main > /dev/null
kill $server
wait $server 2>/dev/null || true
cd "$d/s"
side=$(t log --format=id -n 1 side)
secret=$(blobOf "$d/s" "$side" secret.txt)
mainBlob=$(blobOf "$d/s" "$(t log --format=id -n 1 main)" a.txt)
[ -n "$secret" ] && [ -n "$mainBlob" ] || fail "the blobs' ids"
t branch -D side > /dev/null
cp -R "$d/s" "$d/tpl"

# inside the window the oplog still names side's commit: prune keeps it
t prune --dry-run > "$d/out.txt"
grep -q "^Would remove 0 packed" "$d/out.txt" || fail "a dry run inside the window: $(cat "$d/out.txt")"
t prune > /dev/null
t cat "$secret" > /dev/null || fail "prune removed what undo may need"
echo "ok prune keeps what the oplog names"

# purge: a reachable object is refused, naming the ref; the secret goes from every pack at once
if t purge "$mainBlob" --reason "test" > "$d/out.txt" 2>&1; then
	fail "purge removed a reachable object"
fi
grep -q "refs/heads/main" "$d/out.txt" || fail "the refusal names the ref: $(cat "$d/out.txt")"
t purge "$secret" --reason "a leaked token" > "$d/out.txt"
grep -q "^Purged 1 objects" "$d/out.txt" || fail "purge: $(cat "$d/out.txt")"
if t cat "$secret" > /dev/null 2>&1; then
	fail "the purged blob still reads"
fi
grep -q "$secret a leaked token" .tit/purged || fail "the purge is not recorded"
t log --format=id main > /dev/null || fail "main does not read after the purge"
[ "$(t cat "$mainBlob")" = "one
two" ] || fail "main's blob after the purge"
echo "ok purge refuses what a ref reaches and removes the rest at once"

# past the window: side's commit and tree go; main and a fresh clone are whole
t prune --older-than 0 > "$d/out.txt"
if t cat "$side" > /dev/null 2>&1; then
	fail "side's commit outlived the window: $(cat "$d/out.txt")"
fi
t log --format=id main > /dev/null || fail "main after prune"
(cd "$d/s" && exec "$tit" serve --addr "127.0.0.1:$port") > "$d/serve.log" 2>&1 &
server=$!
k=0
until t clone "$url" "$d/fresh" > /dev/null 2>&1; do
	k=$((k + 1))
	[ $k -lt 100 ] || fail "the server did not start again: $(cat "$d/serve.log")"
	rm -rf "$d/fresh"
	sleep 0.05
done
kill $server
wait $server 2>/dev/null || true
[ "$(cat "$d/fresh/a.txt")" = "one
two" ] || fail "a clone after prune"
echo "ok prune past the window removes the deleted branch; a clone is whole"

# loose objects, and undo inside the window
mkdir "$d/l"
cd "$d/l"
t init . > /dev/null
printf 'a\n' > a.txt
t add a.txt
t commit -m a > /dev/null
t branch x > /dev/null
t switch x > /dev/null
printf 'x\n' > x.txt
t add x.txt
t commit -m x > /dev/null
t switch main > /dev/null
x=$(t log --format=id -n 1 x)
t branch -D x > /dev/null
t prune > /dev/null
t undo --yes > /dev/null
[ "$(t log --format=id -n 1 x)" = "$x" ] || fail "undo after prune"
t branch -D x > /dev/null
t prune --older-than 0 > "$d/out.txt"
if t cat "$x" > /dev/null 2>&1; then
	fail "a loose commit nothing reaches outlived the window: $(cat "$d/out.txt")"
fi
t log --format=id main > /dev/null || fail "main after a loose prune"
echo "ok loose objects, and undo inside the window"

# against git: the objects that survive git gc --prune=now are the ones tit prune keeps
mkdir "$d/g"
cd "$d/g"
export GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=Ada GIT_AUTHOR_EMAIL=ada@example.com GIT_COMMITTER_NAME=Ada GIT_COMMITTER_EMAIL=ada@example.com
git init -q -b main .
printf 'a\n' > a.txt
mkdir sub
printf 's\n' > sub/s.txt
git add -A
git commit -q -m one
printf 'b\n' >> a.txt
git commit -q -am two
git tag -a v1 -m v1
git checkout -q -b dead
printf 'd\n' > dead.txt
mkdir dd
printf 'x\n' > dd/x.txt
git add -A
git commit -q -m dead1
printf 'e\n' >> dead.txt
git commit -q -am dead2
git checkout -q main
"$tit" adopt > /dev/null
git branch -q -D dead
git reflog expire --expire=now --all
git gc -q --prune=now
gitn=$(git count-objects -v | awk '/^in-pack:/ { p = $2 } /^count:/ { c = $2 } END { print p + c }')
"$tit" branch -D dead > /dev/null
titn=$("$tit" prune --older-than 0 | sed -n 's/.* \([0-9]*\) objects are reachable.*/\1/p')
[ "$gitn" = "$titn" ] || fail "git gc keeps $gitn objects, tit prune $titn"
"$tit" log --format=id main > /dev/null || fail "the adopted main after prune"
echo "ok the same $titn objects as git gc --prune=now"

# a crash at every point of a prune past the window: every branch still reads
n=1
while :; do
	rm -rf "$d/w"
	cp -R "$d/tpl" "$d/w"
	set +e
	(cd "$d/w" && TIT_CRASH_AT=$n "$tit" prune --older-than 0 > /dev/null 2>&1)
	rc=$?
	set -e
	if [ $rc -ne 86 ]; then
		[ $rc -eq 0 ] || fail "prune with TIT_CRASH_AT=$n failed with status $rc"
		break
	fi
	(cd "$d/w" && t log --format=id main > /dev/null) || fail "crash $n: main does not read"
	n=$((n + 1))
done
[ $n -gt 1 ] || fail "prune reached no crash point"
echo "ok prune crash: $((n - 1)) crash points, main reads after each"
