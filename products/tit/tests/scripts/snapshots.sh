#!/bin/sh
# Snapshots, timeline, watch and park (#792): a file edited and never committed comes back from timeline after a
# switch; park puts changes aside (untracked files too) and unpark brings them back on the branch they were parked
# on, merged when the branch moved; watch keeps each save; a private key is never kept; old snapshots expire.
# Usage: snapshots.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d" XDG_CONFIG_HOME="$d" TIT_NO_PAGER=1
fail() {
	echo "FAIL snapshots: $*"
	exit 1
}
t() { "$tit" "$@"; }
mkdir -p "$d/r"
cd "$d/r"
t init . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
printf 'one\ntwo\nthree\n' > notes.txt
printf 'keep\n' > other.txt
t add .
t commit -m base > /dev/null
t switch -c side > /dev/null
t switch main > /dev/null

# an edit never committed, carried through a switch, then overwritten: timeline still has it
printf 'one\ntwo\nthree\nthe first draft\n' > notes.txt
t switch side > /dev/null
printf 'one\ntwo\nthree\nthe second draft\n' > notes.txt
t switch main > /dev/null
printf 'one\n' > notes.txt
tl=$(t timeline notes.txt)
[ "$(echo "$tl" | wc -l | tr -d ' ')" = 2 ] || fail "two versions expected: $tl"
echo "$tl" | head -1 | grep -q "before tit switch" || fail "the newest version: $tl"
t timeline notes.txt 2 > /dev/null
grep -qx 'the first draft' notes.txt || fail "version 2 is not the first draft: $(cat notes.txt)"
# putting a version back keeps the file it replaces: that is now version 1
t timeline notes.txt | head -1 | grep -q "before tit timeline" || fail "the replaced file was not kept: $(t timeline notes.txt)"
t timeline notes.txt 1 > /dev/null
[ "$(cat notes.txt)" = one ] || fail "version 1 is not the replaced file: $(cat notes.txt)"
t timeline notes.txt 3 > /dev/null
grep -qx 'the second draft' notes.txt || fail "version 3 is not the second draft: $(cat notes.txt)"
echo "ok a file edited and never committed comes back from timeline after switch"

# park: the changes (an untracked file too) go aside, the branch is remembered
printf 'new file\n' > untracked.txt
t park draft > "$d/park.txt"
grep -q "Parked 2 changed files as draft" "$d/park.txt" || fail "park: $(cat "$d/park.txt")"
[ "$(t status -s)" = "" ] || fail "status after park: $(t status -s)"
[ ! -e untracked.txt ] || fail "the untracked file stayed"
t park | grep -q "^draft  on main at " || fail "park list: $(t park)"
t switch side > /dev/null
t unpark draft > /dev/null
[ "$(cat .tit/workspaces/default/HEAD)" = "ref: refs/heads/main" ] || [ "$(t branch | grep '^\*')" = "* main" ] || fail "unpark did not go back to main: $(t branch)"
grep -qx 'the second draft' notes.txt || fail "the parked edit: $(cat notes.txt)"
[ "$(cat untracked.txt)" = "new file" ] || fail "the parked untracked file"
[ "$(t status -s | sort | tr '\n' ' ')" = " M notes.txt ?? untracked.txt " ] || fail "status after unpark: $(t status -s)"
[ "$(t park)" = "" ] || fail "draft still listed: $(t park)"
echo "ok park and unpark, on the branch it was parked on, unstaged"

# unpark after the branch moved: the parked edit merges with the new commit
t park again > /dev/null
printf 'keep\nand more\n' > other.txt
t commit -am "other grows" > /dev/null
t unpark again > /dev/null
grep -qx 'the second draft' notes.txt && grep -qx 'and more' other.txt || fail "unpark after a commit"
echo "ok unpark merges onto a branch that moved"
t add . > /dev/null
t commit -m drafts > /dev/null

# a private key is never kept
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out key.pem 2> /dev/null
printf 'changed\n' > other.txt
t switch side > /dev/null 2>&1 || true
t timeline other.txt | grep -q "before tit switch" || fail "other.txt was not kept: $(t timeline other.txt)"
t timeline key.pem | grep -q "^No kept version" || fail "the private key was kept: $(t timeline key.pem)"
grep -rq "BEGIN PRIVATE KEY" .tit/workspaces/default/snapshots && fail "a snapshot names the key"
rm key.pem
echo "ok a private key is never kept"

# watch keeps each save
t watch --every 100 --for 3 > "$d/watch.txt" &
w=$!
sleep 1
printf 'watched one\n' > other.txt
sleep 1
printf 'watched two\n' > other.txt
wait $w
tl=$(t timeline other.txt)
echo "$tl" | grep -q "before tit watch" || fail "watch kept nothing: $tl"
t timeline other.txt 1 > /dev/null
grep -qx 'watched two' other.txt || fail "the last save: $(cat other.txt)"
t timeline other.txt 2 > /dev/null
grep -qx 'watched one' other.txt || fail "the save before: $(cat other.txt)"
echo "ok watch keeps each save"

# snapshots older than 14 days go
old=$(( ($(date +%s) - 15 * 24 * 3600) * 1000000000 ))
printf 'tit switch\n' > ".tit/workspaces/default/snapshots/$old"
printf 'again\n' > other.txt
t switch main > /dev/null 2>&1 || true
[ ! -e ".tit/workspaces/default/snapshots/$old" ] || fail "a snapshot of 15 days ago stayed"
echo "ok snapshots expire after 14 days"
