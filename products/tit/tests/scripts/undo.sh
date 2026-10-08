#!/bin/sh
# tit undo and redo (#792), for each kind of operation: doing it, undoing it and redoing it. After the undo the refs
# and HEAD are as before the operation (the files stay, so nothing on the disk is lost); after the redo everything
# is as right after the operation. Usage: undo.sh <tit> <empty directory>
set -eu
tit=$1
d=$2
export HOME="$d" XDG_CONFIG_HOME="$d" TIT_NO_PAGER=1
fail() {
	echo "FAIL undo: $*"
	exit 1
}
t() { "$tit" "$@"; }
refs() {
	echo "HEAD $(sed 's/^ref: //' .tit/workspaces/default/HEAD)"
	for b in $(t branch | sed 's/^[* ] //'); do
		echo "branch $b $(t log --format=id -n 1 "$b")"
	done
	for x in $(t tag); do
		echo "tag $x $(t log --format=id -n 1 "$x")"
	done
}
everything() {
	refs
	t status -s
	for f in $(find . -type f ! -path './.tit/*' | sort); do
		echo "-- $f"
		cat "$f"
	done
}
# check "name" command...: runs the command, undoes it and redoes it
check() {
	name=$1
	shift
	before=$(refs)
	"$@" > /dev/null
	after=$(everything)
	t undo > /dev/null
	[ "$(refs)" = "$before" ] || fail "$name: undo left
$(refs)
not
$before"
	t redo > /dev/null
	[ "$(everything)" = "$after" ] || fail "$name: redo left
$(everything)
not
$after"
	echo "ok $name"
}

mkdir "$d/r"
cd "$d/r"
t init . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
printf 'one\n' > a.txt
t add a.txt
t commit -m first > /dev/null

printf 'two\n' >> a.txt
t add a.txt
check "commit" t commit -m second
[ "$(t status -s)" = "" ] || fail "status after redo"
check "amend" t commit --amend -m "second, amended"
check "branch" t branch side
check "switch -c" t switch -c feature
printf 'feature\n' > f.txt
t add f.txt
check "commit on a branch" t commit -m feature
check "switch" t switch main
check "merge" t merge feature
check "tag" t tag v1
check "annotated tag" t tag -a v2 -m two HEAD~1
check "branch -m" t branch -m side sidetrack
check "branch -d" t branch -d sidetrack
check "detach" t switch --detach HEAD~1
t switch main > /dev/null

# undoing a commit keeps its changes, staged
printf 'three\n' >> a.txt
t commit -am third > /dev/null
t undo > /dev/null
[ "$(t status -s)" = "M  a.txt" ] || fail "undoing a commit: status $(t status -s)"
[ "$(t log --format=subject -n 1)" = "feature" ] || fail "undoing a commit: log"
echo "ok undoing a commit keeps its changes"

# undoing a switch with unsaved changes in the way refuses, and changes nothing
t commit -m third > /dev/null
t switch feature > /dev/null
cp a.txt "$d/a.saved"
printf 'unsaved\n' > a.txt
if t undo > "$d/out.txt" 2>&1; then
	fail "an undo over unsaved changes"
fi
grep -q "uncommitted changes would be overwritten" "$d/out.txt" || fail "undo over unsaved: $(cat "$d/out.txt")"
[ "$(sed 's/^ref: //' .tit/workspaces/default/HEAD)" = "refs/heads/feature" ] || fail "the refused undo moved HEAD"
[ "$(cat a.txt)" = "unsaved" ] || fail "the refused undo touched the file"
echo "ok an undo over unsaved changes refuses"
cp "$d/a.saved" a.txt

# the oplog
t oplog | grep -q "(undone)" || fail "oplog marks undone operations"
t oplog show 2 | grep -q "^operation 2: tit commit" || fail "oplog show: $(t oplog show 2)"
n=$(t oplog | head -1 | cut -d' ' -f1)
t switch main > /dev/null
t branch late > /dev/null
t oplog restore "$n" > /dev/null
t branch | grep -q late && fail "restore left the later branch"
[ "$(sed 's/^ref: //' .tit/workspaces/default/HEAD)" = "refs/heads/feature" ] || fail "restore did not move HEAD back"
[ "$(cat f.txt)" = "feature" ] || fail "restore did not move the files"
echo "ok oplog restore"
# undo --op reverses that operation alone
t branch x > /dev/null
k=$(t oplog | head -1 | cut -d' ' -f1)
t tag y > /dev/null
t undo --op "$k" > /dev/null
t branch | grep -q ' x$' && fail "undo --op left the branch"
[ "$(t tag | grep -c '^y$')" = 1 ] || fail "undo --op took the later tag"
echo "ok undo --op"
