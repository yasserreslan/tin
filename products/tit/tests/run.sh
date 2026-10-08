#!/bin/sh
# tit's tests: each program in tests/programs must print what tests/golden holds. Usage: run.sh [COMPILER]
set -eu
cd "$(dirname "$0")/../../.." || exit 1
compiler=${1:-bin/tinc}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
for src in products/tit/tests/programs/*.tin; do
	name=$(basename "$src" .tin)
	"$compiler" -o "$tmp/$name" "$src"
	"$tmp/$name" > "$tmp/$name.out"
	cmp "$tmp/$name.out" "products/tit/tests/golden/$name.out" || { echo "FAIL tit $name"; exit 1; }
	echo "PASS tit $name"
done
for dir in products/tit/*/; do
	ls "$dir"*_test.tin >/dev/null 2>&1 || continue
	sh ./tin test "$dir" > "$tmp/test.out" 2>&1 || { cat "$tmp/test.out"; echo "FAIL tit tests $dir"; exit 1; }
	echo "PASS tit tests $dir"
done
"$compiler" -o "$tmp/tit" products/tit/main.tin
"$tmp/tit" version -v > "$tmp/version.out"
cmp "$tmp/version.out" products/tit/tests/golden/version.out || { echo "FAIL tit version"; exit 1; }
echo "PASS tit version"
