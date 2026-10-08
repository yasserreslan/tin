#!/bin/sh
# A session of saving work (#790), run the same way with git and with tit: the output of status --short, diff,
# diff --staged and diff --stat after each step must be the same (index lines, which carry ids, are left out).
# Usage: save.sh <git|path to tit> <empty directory>
set -eu
vcs=$1
d=$2
cd "$d"
if [ "$vcs" = git ]; then
	git init -q -b main .
	git config user.name Ada
	git config user.email ada@example.com
	git config core.quotepath off
	git config diff.renames true
	v() { git "$@"; }
	commit() { git commit -q "$@"; }
else
	"$vcs" init . > /dev/null
	"$vcs" config set user.name Ada
	"$vcs" config set user.email ada@example.com
	v() { TIT_NO_PAGER=1 "$vcs" "$@"; }
	commit() { v commit "$@" > /dev/null; }
fi
show() {
	echo "== $1"
	v status --short
	echo "-- diff"
	v diff | sed '/^index /d'
	echo "-- staged"
	v diff --staged | sed '/^index /d'
	echo "-- stat"
	v diff --staged --stat
}

printf 'one\ntwo\nthree\n' > a.txt
printf '#!/bin/sh\necho hi\n' > run.sh
mkdir -p src/deep
printf 'package main\n' > src/main.tin
printf 'deep\n' > src/deep/x.txt
printf '*.log\nbuild/\n' > .gitignore
printf 'noise\n' > debug.log
mkdir build && printf 'out\n' > build/out.bin
show "a new directory"
v add .
show "everything staged"
commit -m "first"
show "after the first commit"

printf 'one\n2\nthree\nfour\n' > a.txt
chmod +x run.sh
printf 'new\n' > b.txt
rm src/deep/x.txt
show "edits on the disk"
v add a.txt b.txt
show "two staged"
commit -a -m "second" -m "with a body"
show "after commit -a"

v mv a.txt renamed.txt > /dev/null
v rm src/main.tin > /dev/null
mkdir -p docs && printf 'guide\n' > docs/guide.txt
show "a move, a removal and an untracked directory"
v add docs
printf 'guide, edited\n' > docs/guide.txt
show "staged, then edited again"
commit -m "third"
v rm --cached b.txt > /dev/null
show "rm --cached"
printf 'binary\000data\n' > img.bin
v add img.bin
show "a binary file"
commit -m "fourth"
printf 'four\n' > b.txt
v add b.txt
commit --amend -m "fourth, amended"
show "after an amend"
v mv run.sh tools.sh > /dev/null
printf 'echo more\n' >> tools.sh
v add tools.sh
show "a move with an edit"
commit -m "fifth"
echo "== log"
v log --oneline | sed 's/^[0-9a-f]* //'
