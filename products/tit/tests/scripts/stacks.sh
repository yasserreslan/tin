#!/bin/sh
# Stacks (#793, #786): absorb puts each fix in the change that wrote those lines; a hunk no change owns stays; move
# reorders, and across a change editing the same lines records the conflict without stopping; edit puts the
# markers back and amend rebases what is above; split; one undo reverses a move. Usage: stacks.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d" XDG_CONFIG_HOME="$d" TIT_NO_PAGER=1
fail() {
	echo "FAIL stacks: $*"
	exit 1
}
t() { "$tit" "$@"; }
lines() {
	i=1
	while [ $i -le 10 ]; do
		echo "$1 line $i"
		i=$((i + 1))
	done
}
mkdir "$d/r"
cd "$d/r"
t init . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
for f in a b c d; do lines $f > $f.txt; done
t add .
t commit -m base > /dev/null
t switch -c feat > /dev/null
sed 's/^a line 2$/a line 2, changed by one/' a.txt > x && mv x a.txt && t commit -am one > /dev/null
sed 's/^b line 5$/b line 5, changed by two/' b.txt > x && mv x b.txt && t commit -am two > /dev/null
sed 's/^c line 7$/c line 7, changed by three/' c.txt > x && mv x c.txt && t commit -am three > /dev/null
ids=$(t log --format=change -n 3 | tr '\n' ' ')
one=$(t log --format=change -n 1 HEAD~2)
two=$(t log --format=change -n 1 HEAD~1)
three=$(t log --format=change -n 1)
[ "$(t stack | tail -3 | awk '{print $1}' | tr '\n' ' ')" = "$(echo $one | cut -c1-12) $(echo $two | cut -c1-12) $(echo $three | cut -c1-12) " ] || fail "stack: $(t stack)"
echo "ok stack lists the three changes, oldest first"

# fixes for each change, and one for a file no change touched
sed 's/^a line 2, changed by one$/a line 2, changed by one (fixed)/' a.txt > x && mv x a.txt
sed 's/^b line 5, changed by two$/b line 5, changed by two (fixed)/' b.txt > x && mv x b.txt
sed 's/^c line 7, changed by three$/c line 7, changed by three (fixed)/' c.txt > x && mv x c.txt
sed 's/^d line 1$/d line 1, unowned/' d.txt > x && mv x d.txt
t absorb > "$d/out.txt"
grep -q "^Absorbed 3 hunks into 3 changes\.$" "$d/out.txt" || fail "absorb said: $(cat "$d/out.txt")"
grep -q "stays: d.txt:1" "$d/out.txt" || fail "the unowned hunk: $(cat "$d/out.txt")"
t diff "$one~1" "$one" | grep -q '^+a line 2, changed by one (fixed)$' || fail "one did not take its fix"
t diff "$two~1" "$two" | grep -q '^+b line 5, changed by two (fixed)$' || fail "two did not take its fix"
t diff "$three~1" "$three" | grep -q '^+c line 7, changed by three (fixed)$' || fail "three did not take its fix"
[ "$(t status -s)" = " M d.txt" ] || fail "status after absorb: $(t status -s)"
[ "$(t log --format=change -n 3 | tr '\n' ' ')" = "$ids" ] || fail "absorb changed the change ids"
t diff d.txt > /dev/null
sed 's/^d line 1, unowned$/d line 1/' d.txt > x && mv x d.txt
echo "ok absorb put every fix in its change and left the unowned hunk"

t move "$three" --before "$one" > /dev/null
[ "$(t stack | tail -3 | awk '{print $1}' | tr '\n' ' ')" = "$(echo $three | cut -c1-12) $(echo $one | cut -c1-12) $(echo $two | cut -c1-12) " ] || fail "after move: $(t stack)"
grep -q 'changed by three (fixed)' c.txt || fail "the files after move"
t undo > /dev/null
[ "$(t stack | tail -3 | awk '{print $1}' | tr '\n' ' ')" = "$(echo $one | cut -c1-12) $(echo $two | cut -c1-12) $(echo $three | cut -c1-12) " ] || fail "undo of move: $(t stack)"
echo "ok move, and one undo reverses it"

# a change that edits the line one edits, moved below one: its own edit no longer applies, so its new version records
# the conflict, and the rebase goes on through one, two and three
sed 's/^a line 2, changed by one (fixed)$/a line 2, rewritten by four/' a.txt > x && mv x a.txt
t commit -am four > /dev/null
four=$(t log --format=change -n 1)
t move "$four" --before "$one" > "$d/out.txt" || fail "the move stopped: $(cat "$d/out.txt")"
t stack | grep "^  $(echo $four | cut -c1-12)" | grep -q "conflicts: a.txt" || fail "no conflict recorded: $(t stack)"
[ "$(t stack | tail -4 | awk '{print $1}' | tr '\n' ' ')" = "$(echo $four | cut -c1-12) $(echo $one | cut -c1-12) $(echo $two | cut -c1-12) $(echo $three | cut -c1-12) " ] || fail "the rebase stopped: $(t stack)"
echo "ok a move across the same lines records the conflict and goes on"

t edit "$four" > /dev/null
grep -q '^<<<<<<< HEAD$' a.txt || fail "edit did not write markers: $(cat a.txt)"
[ "$(t status -s | head -1)" = "UU a.txt" ] || fail "edit status: $(t status -s)"
# four gives up its edit of line 2, so one's edit applies above it again
lines a > a.txt
t add a.txt
t commit --amend > /dev/null
t stack | grep -q conflicts && fail "conflicts left after the amend: $(t stack)"
[ "$(sed 's/^ref: //' .tit/workspaces/default/HEAD)" = "refs/heads/feat" ] || fail "not back on feat"
grep -q '^a line 2, changed by one (fixed)$' a.txt || fail "the tip does not have one's edit: $(cat a.txt)"
[ "$(t status -s)" = "" ] || fail "status after the edit: $(t status -s)"
echo "ok edit, resolve, amend: the changes above follow"

printf 'x\n' > x.txt
printf 'y\n' > y.txt
t add x.txt y.txt
t commit -m "x and y" > /dev/null
xy=$(t log --format=change -n 1)
t split "$xy" x.txt -m "just x" > /dev/null
[ "$(t log --format=subject -n 2 | tr '\n' '|')" = "x and y|just x|" ] || fail "split: $(t log --format=subject -n 2)"
t diff HEAD~1 HEAD --stat | grep -q 'y.txt' || fail "the rest did not keep y.txt"
t diff HEAD~1 HEAD --stat | grep -q 'x.txt' && fail "the rest kept x.txt"
echo "ok split"
