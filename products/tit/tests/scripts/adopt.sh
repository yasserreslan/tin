#!/bin/sh
# tit adopt against git itself (#789): the logs of every branch and tag equal git's commit for commit, the working
# directory is clean for both, a second adopt takes only the new commit, a git worktree adopts, a shallow clone is
# refused, an adopt cut by SIGTERM finishes when run again without redoing its finished packs, two adopts write
# byte-identical packs, and an adopt in several batches ends with three packs (one of each kind, #807) and the same
# history as one batch, while an adopt of one batch adds its three. Usage: adopt.sh <tit> <empty directory>
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
same "packs after two adopts of one batch each" "$(ls .tit/packs | grep -c '\.pack$')" 6

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

# an adopt cut by SIGTERM finishes when run again, keeping the packs it had finished; and two adopts of one repository,
# one on a single core and one on all of them, write byte-identical packs (each pack is named by its hash)
mkdir "$d/big"
cd "$d/big"
git init -q -b main .
perl -e 'srand(7); for $i (1..160) { $c = join("", map { chr(32 + int(rand(95))) } 1..100000) . "\n"; $m = "c$i\n"; print "commit refs/heads/main\ncommitter A <a\@b> " . (1700000000 + $i) . " +0000\ndata " . length($m) . "\n$m"; print "M 100644 inline f$i.txt\ndata " . length($c) . "\n$c\n"; }' | git fast-import --quiet
git checkout -q main
cp -R "$d/big" "$d/big2"
TIT_ADOPT_BATCH_MB=2 perl -e '$SIG{TERM} = "DEFAULT"; exec @ARGV' "$tit" adopt > /dev/null 2>&1 &
pid=$!
n=0
# a batch is finished once its pairs of ids are in the git-ids table
until [ -s .tit/git-ids ] && [ "$(ls .tit/packs 2> /dev/null | grep -c '\.pack$')" -ge 2 ]; do
	n=$((n + 1))
	[ $n -lt 300 ] || fail "adopt wrote no pack to interrupt"
	sleep 0.02
done
kill -TERM $pid 2> /dev/null || true
wait $pid 2> /dev/null || true
before=$(ls .tit/packs | grep '\.pack$' | sort)
total=$(git rev-list --all --objects | wc -l | tr -d ' ')
TIT_ADOPT_BATCH_MB=2 "$tit" adopt > "$d/again.txt" 2>&1 || fail "adopt after SIGTERM: $(cat "$d/again.txt")"
again=$(sed -n 's/^Adopted \([0-9]*\) objects.*/\1/p' "$d/again.txt")
[ -n "$again" ] && [ "$again" -lt "$total" ] || fail "the second adopt redid everything: $(cat "$d/again.txt") of $total"
for p in $before; do
	[ -e ".tit/packs/$p" ] || fail "the pack $p finished before SIGTERM is gone"
done
same "log after SIGTERM" "$("$tit" log --format=git-id)" "$(git log --format=%H)"
echo "ok an adopt cut by SIGTERM finishes when run again: $again of $total objects the second time"
cd "$d/big2"
TIT_ADOPT_BATCH_MB=2 TIN_CORES=1 "$tit" adopt > /dev/null
rm -rf "$d/big3"
cp -R "$d/big2" "$d/big3"
rm -rf "$d/big3/.tit"
cd "$d/big3"
TIT_ADOPT_BATCH_MB=2 "$tit" adopt > /dev/null
[ "$(ls "$d/big2/.tit/packs" | sort)" = "$(ls "$d/big3/.tit/packs" | sort)" ] || fail "two adopts wrote different packs"
echo "ok two adopts (one core, all cores) write byte-identical packs"

# several batches end as three packs, one of each kind (#807), with the objects and history of one batch
packs() {
	ls "$1/.tit/packs" | grep -c '\.pack$'
}
same "packs after an adopt in several batches" "$(packs "$d/big3")" 3
same "log after an adopt in several batches" "$("$tit" log --format=git-id)" "$(git log --format=%H)"
rm -rf "$d/big4"
cp -R "$d/big3" "$d/big4"
rm -rf "$d/big4/.tit"
cd "$d/big4"
"$tit" adopt > /dev/null
same "packs after an adopt in one batch" "$(packs "$d/big4")" 3
same "ids of one batch and of several" "$(cd "$d/big3" && "$tit" log --format=id)" "$("$tit" log --format=id)"
same "diff of one batch and of several" "$(cd "$d/big3" && "$tit" diff HEAD~150 HEAD | cksum)" "$("$tit" diff HEAD~150 HEAD | cksum)"
