#!/bin/sh
# tit adopt against git itself (#789): the logs of every branch and tag equal git's commit for commit, the working
# directory is clean for both, a second adopt takes only the new commit, a git worktree adopts, a shallow clone is
# refused. Usage: adopt.sh <tit> <empty directory>
set -eu
tit=$1
d=$2
export GIT_CONFIG_NOSYSTEM=1 HOME="$d" TIT_NO_PAGER=1
export GIT_AUTHOR_NAME="Ada Lovelace" GIT_AUTHOR_EMAIL=ada@example.com GIT_COMMITTER_NAME="Grace Hopper" GIT_COMMITTER_EMAIL=grace@example.com
fail() {
	echo "FAIL adopt: $*"
	exit 1
}
same() {
	[ "$2" = "$3" ] || fail "$1: tit says
$2
git says
$3"
	echo "ok $1"
}

g="$d/repo"
mkdir "$g"
cd "$g"
git init -q -b main .
n=0
commit() {
	n=$((n + 1))
	GIT_AUTHOR_DATE="$((1700000000 + n * 60)) +0200" GIT_COMMITTER_DATE="$((1700000000 + n * 60)) +0200" git commit -q "$@"
}
printf 'one\n' > a.txt
mkdir src && printf 'package main\n' > src/main.tin
git add -A && commit -m first
printf 'two\n' >> a.txt && commit -am second
git checkout -q -b side
printf 'side\n' > side.txt && git add side.txt && commit -m side
git checkout -q main
printf 'three\n' >> a.txt && commit -am third
GIT_AUTHOR_DATE="1700009999 +0200" GIT_COMMITTER_DATE="1700009999 +0200" git merge -q --no-edit side
git tag -a v1 -m "version one" HEAD~1
git gc -q

"$tit" adopt > "$d/out.txt"
grep -q "^Adopted .* from .*; 3 refs moved\.$" "$d/out.txt" || fail "adopt said: $(cat "$d/out.txt")"
for rev in main side v1; do
	same "log $rev" "$("$tit" log --format=git-id "$rev")" "$(git log --format=%H "$rev")"
done
same "first-parent log" "$("$tit" log --first-parent --format=git-id)" "$(git log --first-parent --format=%H)"
same "tit status" "$("$tit" status -s)" ""
same "git status" "$(git status --porcelain)" ""
id=$(git rev-parse HEAD~2 | cut -c1-10)
same "show by git id" "$("$tit" show "$id" | sed -n 's/^    //p' | head -1)" "$(git log -1 --format=%s HEAD~2)"

printf 'four\n' >> a.txt && commit -am fourth
same "a second adopt" "$("$tit" adopt | head -1 | sed 's/ from .*;/;/')" "Adopted 3 objects (1 commit or tag); 1 ref moved."
same "a third" "$("$tit" adopt | sed 's/ in .*//')" "Nothing new"
same "log after" "$("$tit" log --format=git-id)" "$(git log --format=%H)"

git worktree add -q "$d/wt" side
cd "$d/wt"
"$tit" adopt > /dev/null
same "worktree log" "$("$tit" log --format=git-id)" "$(git log --format=%H)"

cd "$d"
git clone -q --depth 1 "file://$g" shallow
cd shallow
if "$tit" adopt > "$d/shallow.txt" 2>&1; then
	fail "a shallow clone was adopted"
fi
grep -q "unshallow" "$d/shallow.txt" || fail "shallow: $(cat "$d/shallow.txt")"
echo "ok shallow clone refused"
