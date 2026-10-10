#!/bin/sh
# Regenerate golden/treediff_git.out from git: write the two directories with programs/treediff_git.tin, commit the
# first in a repository, put the second in its place, and ask git for the staged changes (--name-status, with rename
# scores dropped, and --stat). Needs git. Usage: treediff_git.sh [COMPILER]
set -eu
cd "$(dirname "$0")/../../.." || exit 1
compiler=${1:-bin/tinc}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
# the program works in this directory and leaves its files there (#1065)
export TIN_TEST_DIR="$tmp/work"
"$compiler" -o "$tmp/treediff_git" products/tit/tests/programs/treediff_git.tin
"$tmp/treediff_git" setup
out=$PWD/products/tit/tests/golden/treediff_git.out
repo=$tmp/repo
mkdir "$repo"
cp -R "$TIN_TEST_DIR"/a/. "$repo/"
cd "$repo"
git init -q
git add -A
git -c user.name=t -c user.email=t@t commit -qm a
find . -mindepth 1 -maxdepth 1 ! -name .git -exec rm -rf {} +
cp -R "$TIN_TEST_DIR"/b/. .
git add -A
{
	git diff --cached -M --name-status | sed 's/^R[0-9]*/R/'
	git diff --cached -M --stat=80
} > "$out"
echo "wrote $out"
