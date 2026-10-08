#!/bin/sh
# Branches and merging (#791), run the same way with git and with tit: after each step the branch list, status
# --short, every file's content and the log's subjects must be the same.
# Usage: branch.sh <git|path to tit> <empty directory>
set -eu
vcs=$1
d=$2
cd "$d"
n=0
if [ "$vcs" = git ]; then
	git init -q -b main .
	git config user.name Ada
	git config user.email ada@example.com
	git config merge.suppressDest '*'
	git config merge.conflictStyle merge
	v() { git "$@"; }
	commit() {
		n=$((n + 1))
		GIT_AUTHOR_DATE="$((1700000000 + n * 60)) +0000" GIT_COMMITTER_DATE="$((1700000000 + n * 60)) +0000" git commit -q "$@"
	}
	merge() {
		n=$((n + 1))
		GIT_AUTHOR_DATE="$((1700000000 + n * 60)) +0000" GIT_COMMITTER_DATE="$((1700000000 + n * 60)) +0000" git merge -q --no-edit "$@" > /dev/null 2>&1
	}
	switch() { git switch -q "$@"; }
else
	"$vcs" init . > /dev/null
	"$vcs" config set user.name Ada
	"$vcs" config set user.email ada@example.com
	v() { TIT_NO_PAGER=1 "$vcs" "$@"; }
	commit() {
		n=$((n + 1))
		TIT_TIME=$((1700000000 + n * 60)) v commit "$@" > /dev/null
	}
	merge() {
		n=$((n + 1))
		TIT_TIME=$((1700000000 + n * 60)) v merge "$@" > /dev/null 2>&1
	}
	switch() { v switch "$@" > /dev/null; }
fi
show() {
	echo "== $1"
	v branch
	v status --short
	for f in $(find . -type f ! -path './.git/*' ! -path './.tit/*' | sort); do
		echo "-- $f"
		cat "$f"
	done
	echo "-- log"
	v log --format=subject 2>/dev/null || v log --format=%s
}

printf 'one\ntwo\nthree\n' > a.txt
printf 'b\n' > b.txt
v add a.txt b.txt
commit -m "first"
v branch side
switch side
printf 'one\ntwo\nthree\nfour from side\n' > a.txt
printf 'side\n' > side.txt
v add a.txt side.txt
commit -m "on the side"
switch main
printf 'zero from main\none\ntwo\nthree\n' > a.txt
commit -am "on main"
show "two branches"
merge side
show "a clean merge"

switch -c ff
printf 'ff\n' > ff.txt
v add ff.txt
commit -m "fast-forward me"
switch main
merge ff
show "a fast-forward"

v branch -d ff > /dev/null
v branch -m side sidetrack > /dev/null
show "a branch deleted, one renamed"

switch -c left
printf 'zero from main\none\nTWO from left\nthree\nfour from side\n' > a.txt
commit -am "left"
switch main
printf 'zero from main\none\nTWO from main\nthree\nfour from side\n' > a.txt
commit -am "main again"
merge left || true
show "a conflict"
v merge --abort
show "abandoned"
merge left || true
printf 'zero from main\none\ntwo, both\nthree\nfour from side\n' > a.txt
v add a.txt
commit -m "resolved"
show "resolved"

v tag v1
v tag -a v2 -m "version two" HEAD~1
echo "== tags"
v tag
