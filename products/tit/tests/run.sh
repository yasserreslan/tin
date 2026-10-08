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
# every place a secret (a private key) leaves the checker's protection is reviewed: a new one fails here
sh ./tin audit secrets products/tit/main.tin 2>&1 | sed 's/:[0-9]*:[0-9]*:/:/' > "$tmp/reveals.txt"
cmp "$tmp/reveals.txt" products/tit/tests/reveals.txt || { diff "$tmp/reveals.txt" products/tit/tests/reveals.txt; echo "FAIL tit reveals: review the change, then update products/tit/tests/reveals.txt"; exit 1; }
echo "PASS tit reveals"
"$compiler" -o "$tmp/tit" products/tit/main.tin
"$tmp/tit" version -v > "$tmp/version.out"
cmp "$tmp/version.out" products/tit/tests/golden/version.out || { echo "FAIL tit version"; exit 1; }
echo "PASS tit version"
# scripted sessions: the same commands as git's, with the same output (golden/session_*.out, from sessions/golden.sh)
for s in products/tit/tests/sessions/*.sh; do
	name=$(basename "$s" .sh)
	[ "$name" = golden ] && continue
	mkdir "$tmp/session_$name" "$tmp/home_$name"
	HOME="$tmp/home_$name" XDG_CONFIG_HOME="$tmp/home_$name" TZ=UTC sh "$s" "$tmp/tit" "$tmp/session_$name" > "$tmp/session_$name.out" 2>&1 || { cat "$tmp/session_$name.out"; echo "FAIL tit session $name"; exit 1; }
	cmp "$tmp/session_$name.out" "products/tit/tests/golden/session_$name.out" || { diff "products/tit/tests/golden/session_$name.out" "$tmp/session_$name.out"; echo "FAIL tit session $name"; exit 1; }
	echo "PASS tit session $name"
done
# scripts that check tit against git themselves (they need git) and exit non-zero on a difference
for s in products/tit/tests/scripts/*.sh; do
	name=$(basename "$s" .sh)
	mkdir "$tmp/script_$name"
	sh "$s" "$tmp/tit" "$tmp/script_$name" > "$tmp/script_$name.out" 2>&1 || { cat "$tmp/script_$name.out"; echo "FAIL tit script $name"; exit 1; }
	echo "PASS tit script $name"
done
