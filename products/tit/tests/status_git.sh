#!/bin/sh
# Regenerate golden/status_git.out from git itself: the same file changes as programs/status_git.tin, with git doing
# the staging, then `git status --porcelain`. Needs git. Usage: status_git.sh [COMPILER]
set -eu
cd "$(dirname "$0")/../../.." || exit 1
compiler=${1:-bin/tinc}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
"$compiler" -o "$tmp/status_git" products/tit/tests/programs/status_git.tin
d=/tmp/tin-test-tit-status
"$tmp/status_git" init
(cd "$d" && git init -q && git add -A && git -c user.name=t -c user.email=t@t commit -qm init)
"$tmp/status_git" stage-files
(cd "$d" && git add f.txt g.txt i.txt && git rm -q --cached h.txt)
"$tmp/status_git" work
(cd "$d" && git -c core.quotepath=off status --porcelain --untracked-files=all) > products/tit/tests/golden/status_git.out
echo "wrote products/tit/tests/golden/status_git.out"
