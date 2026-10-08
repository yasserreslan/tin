#!/bin/sh
# tit add -p (#790): the same file, the same edits and the same answers given to git add -p and to tit add -p
# show the same hunks and stage the same lines: the staged and the unstaged diffs are git's (ids aside). Then a, d, q
# and a deletion. Usage: addp.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d" XDG_CONFIG_HOME="$d" TIT_NO_PAGER=1 GIT_PAGER=cat
fail() {
	echo "FAIL addp: $*"
	exit 1
}
t() { "$tit" "$@"; }
lines() {
	i=1
	while [ $i -le 40 ]; do
		echo "line $i"
		i=$((i + 1))
	done
}
edit() {
	sed -e 's/^line 3$/line 3, edited/' -e 's/^line 20$/line 20, edited/' -e '/^line 30$/d' -e 's/^line 40$/line 40\nline 41/' f.txt > x
	mv x f.txt
	printf 'other\nchanged\n' > g.txt
}
# no index lines, which name ids
body() {
	grep -v '^index ' || true
}
mkdir "$d/git" "$d/tit"
for side in git tit; do
	cd "$d/$side"
	lines > f.txt
	printf 'other\n' > g.txt
	if [ $side = git ]; then
		git init -q .
		git add .
		git -c user.name=Ada -c user.email=ada@example.com commit -qm base
	else
		t init . > /dev/null
		t config set user.name Ada
		t config set user.email ada@example.com
		t add .
		t commit -m base > /dev/null
	fi
	edit
done
answers='y
n
y
n
n
'
cd "$d/git"
printf '%s' "$answers" | git add -p > "$d/git.out" 2>&1
cd "$d/tit"
printf '%s' "$answers" | t add -p > "$d/tit.out" 2>&1
grep '^@@' "$d/git.out" > "$d/git.hunks"
grep '^@@' "$d/tit.out" > "$d/tit.hunks"
cmp -s "$d/git.hunks" "$d/tit.hunks" || fail "the hunks differ:
$(diff "$d/git.hunks" "$d/tit.hunks")"
(cd "$d/git" && git diff --cached | body) > "$d/git.staged"
t diff --staged | body > "$d/tit.staged"
cmp -s "$d/git.staged" "$d/tit.staged" || fail "the staged diffs differ:
$(diff "$d/git.staged" "$d/tit.staged")"
(cd "$d/git" && git diff | body) > "$d/git.unstaged"
t diff | body > "$d/tit.unstaged"
cmp -s "$d/git.unstaged" "$d/tit.unstaged" || fail "the unstaged diffs differ:
$(diff "$d/git.unstaged" "$d/tit.unstaged")"
[ "$(t status -s)" = "$(cd "$d/git" && git status -s)" ] || fail "status: $(t status -s)"
echo "ok add -p stages what git add -p stages"

# a stages the rest of the file, d leaves it, q stops
t commit -m "the first part" > /dev/null
printf 'a\n' | t add -p f.txt > /dev/null
[ "$(t diff -- f.txt 2> /dev/null || t diff f.txt)" = "" ] || fail "a left hunks of f.txt: $(t diff f.txt)"
t commit -m "the rest of f" > /dev/null
printf 'd\n' | t add -p > /dev/null
[ "$(t status -s)" = " M g.txt" ] || fail "d staged something: $(t status -s)"
printf 'q\n' | t add -p > /dev/null
[ "$(t status -s)" = " M g.txt" ] || fail "q staged something: $(t status -s)"
rm g.txt
printf 'y\n' | t add -p > /dev/null
[ "$(t status -s)" = "D  g.txt" ] || fail "the deletion: $(t status -s)"
echo "ok a, d, q and a deletion"
