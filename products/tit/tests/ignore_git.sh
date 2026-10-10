#!/bin/sh
# Regenerate golden/ignore_git.out from git itself: build the tree with programs/ignore_git.tin, then ask git which
# files it ignores. Needs git. Usage: ignore_git.sh [COMPILER]
set -eu
cd "$(dirname "$0")/../../.." || exit 1
compiler=${1:-bin/tinc}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
# the program works in this directory and leaves its files there (#1065)
export TIN_TEST_DIR="$tmp/work"
"$compiler" -o "$tmp/ignore_git" products/tit/tests/programs/ignore_git.tin
"$tmp/ignore_git" > /dev/null
(cd "$TIN_TEST_DIR" && git init -q . && git -c core.excludesFile=/dev/null ls-files -o -i --exclude-standard) | LC_ALL=C sort > products/tit/tests/golden/ignore_git.out
echo "wrote products/tit/tests/golden/ignore_git.out"
