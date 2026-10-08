#!/bin/sh
# Workspaces (#804, #792): a workspace is its own working directory, branch, HEAD and undo on the same store; undo in
# one does not reach the other, and refuses when the other moved the branch since; two workspaces committing at once
# leave every commit and operation; who counts the changes to a path. Usage: workspaces.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d" XDG_CONFIG_HOME="$d" TIT_NO_PAGER=1
fail() {
	echo "FAIL workspaces: $*"
	exit 1
}
t() { "$tit" "$@"; }
mkdir "$d/r"
cd "$d/r"
t init . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
printf 'one\n' > a.txt
t add a.txt
t commit -m first > /dev/null

t workspace new feat > /dev/null
[ -f "$d/r-feat/a.txt" ] || fail "the workspace's files"
[ "$(t -C "$d/r-feat" status -s)" = "" ] || fail "the new workspace is not clean"
t workspaces | grep -q "feat	feat	$d/r-feat" || fail "workspaces: $(t workspaces)"
echo "ok a new workspace"

# each workspace's undo is its own
printf 'main\n' >> a.txt
t commit -am "on main" > /dev/null
printf 'feat\n' > "$d/r-feat/f.txt"
t -C "$d/r-feat" add f.txt
t -C "$d/r-feat" commit -m "on feat" > /dev/null
main=$(t log --format=id -n 1 main)
t -C "$d/r-feat" undo > /dev/null
[ "$(t log --format=subject -n 1 feat)" = "first" ] || fail "undo in feat did not move feat back"
[ "$(t log --format=id -n 1 main)" = "$main" ] || fail "undo in feat moved main"
t -C "$d/r-feat" redo > /dev/null
echo "ok undo stays in its workspace"

# a second workspace on the same branch commits on top: the first's undo refuses and names it
t workspace new other > /dev/null
t -C "$d/r-other" switch feat > /dev/null
printf 'on top\n' >> "$d/r-other/f.txt"
t -C "$d/r-other" commit -am "on top, from other" > /dev/null
if t -C "$d/r-feat" undo > "$d/out.txt" 2>&1; then
	fail "undo in feat went through after other moved feat"
fi
grep -q "workspace other" "$d/out.txt" || fail "the refusal: $(cat "$d/out.txt")"
echo "ok undo refuses and names the other workspace"

# two workspaces committing at once
t workspace new w1 > /dev/null
t workspace new w2 > /dev/null
before=$(t oplog | wc -l)
for w in w1 w2; do
	(
		i=0
		while [ $i -lt 40 ]; do
			printf '%s\n' $i > "$d/r-$w/$w.txt"
			t -C "$d/r-$w" add "$w.txt"
			t -C "$d/r-$w" commit -m "$w $i" > /dev/null
			i=$((i + 1))
		done
	) &
done
wait
[ "$(t log --format=subject w1 | grep -c '^w1 ')" = 40 ] || fail "w1 lost commits"
[ "$(t log --format=subject w2 | grep -c '^w2 ')" = 40 ] || fail "w2 lost commits"
[ $(($(t oplog | wc -l) - before)) = 80 ] || fail "operations: $(($(t oplog | wc -l) - before))"
# the shared store is whole: every commit of both reads back, with the file it wrote
for w in w1 w2; do
	k=39
	for id in $(t log --format=id $w -n 40); do
		t show "$id" | grep -q "^+$k$" || fail "$w's commit $id does not read back with $k"
		k=$((k - 1))
	done
done
[ "$(t oplog | cut -d' ' -f1 | sort | uniq -d)" = "" ] || fail "an operation number twice"
echo "ok two workspaces committing at once"

t who a.txt | grep -q "^Ada <ada@example.com>  2 changes" || fail "who: $(t who a.txt)"
echo "ok who"

printf 'unsaved\n' >> "$d/r-w1/w1.txt"
t workspace rm w1 > "$d/out.txt" 2>&1 && fail "rm took a workspace with changes"
t workspace rm w2 > /dev/null
[ ! -e "$d/r-w2" ] || fail "rm left the files"
t workspaces | cut -f1 | grep -q "w2$" && fail "rm left the workspace"
t branch | grep -q " w2$" || fail "rm took the branch"
echo "ok workspace rm"
