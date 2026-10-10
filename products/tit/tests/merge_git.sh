#!/bin/sh
# Regenerate golden/merge_git.out from git itself: write the triples with programs/merge_git.tin, then merge each with
# git merge-file. Needs git and shasum. Usage: merge_git.sh [COMPILER]
set -eu
cd "$(dirname "$0")/../../.." || exit 1
compiler=${1:-bin/tinc}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
# the program works in this directory and leaves its files there (#1065)
export TIN_TEST_DIR="$tmp/work"
"$compiler" -o "$tmp/merge_git" products/tit/tests/programs/merge_git.tin
"$tmp/merge_git" > /dev/null
out=products/tit/tests/golden/merge_git.out
: > "$out"
k=0
while [ $k -lt 60 ]; do
	d=$TIN_TEST_DIR
	if git merge-file -p "$d/$k.ours" "$d/$k.base" "$d/$k.theirs" > "$tmp/m" 2>/dev/null; then
		echo "$k clean $(shasum -a 256 "$tmp/m" | cut -d' ' -f1)" >> "$out"
	else
		echo "$k conflict" >> "$out"
	fi
	k=$((k + 1))
done
echo "wrote $out"
