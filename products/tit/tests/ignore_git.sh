#!/bin/sh
# Regenerate golden/ignore_git.out from git itself: build the tree with programs/ignore_git.tin, then ask git which
# files it ignores. Needs git. Usage: ignore_git.sh [COMPILER]
set -eu
cd "$(dirname "$0")/../../.." || exit 1
compiler=${1:-bin/tinc}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
"$compiler" -o "$tmp/ignore_git" products/tit/tests/programs/ignore_git.tin
"$tmp/ignore_git" > /dev/null
(cd /tmp/tin-test-tit-ignore && git init -q . && git -c core.excludesFile=/dev/null ls-files -o -i --exclude-standard) | LC_ALL=C sort > products/tit/tests/golden/ignore_git.out
echo "wrote products/tit/tests/golden/ignore_git.out"
