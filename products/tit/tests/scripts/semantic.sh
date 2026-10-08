#!/bin/sh
# diff --semantic and history (#798): a reformatted file is no change, a function moved to another file shows as
# moved, a renamed one as renamed, and history follows a declaration through its rename and its move.
# Usage: semantic.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d" XDG_CONFIG_HOME="$d" TIT_NO_PAGER=1
fail() {
	echo "FAIL semantic: $*"
	exit 1
}
t() { "$tit" "$@"; }
root=$PWD
mkdir -p "$d/r/geo"
cd "$d/r"
t init . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
cat > geo/a.tin <<'X'
package geo

// Area is the area of a rectangle.
fn Area(w i64, h i64) i64 {
	return w * h
}

fn helper(x i64) i64 {
	let y = x + 1
	return y * 2
}
X
printf 'package geo\n' > geo/b.tin
t add .
t commit -m "geo" > /dev/null
# a reformat: spacing and a blank line only
perl -pi -e 's/return w \* h/return   w * h/' geo/a.tin
[ "$(t diff --semantic)" = "" ] || fail "a reformat shows: $(t diff --semantic)"
echo "ok a reformat is no change"
t commit -am "reformat" > /dev/null
# Area moves to b.tin and changes; helper is renamed
cat > geo/a.tin <<'X'
package geo

fn twice(x i64) i64 {
	let y = x + 1
	return y * 2
}
X
cat > geo/b.tin <<'X'
package geo

// Area is the area of a rectangle.
fn Area(w i64, h i64) i64 {
	return w * h * 1
}
X
out=$(t diff --semantic)
echo "$out" | grep -q "moved fn Area (from geo/a.tin)" || fail "moved: $out"
echo "$out" | grep -q "changed fn Area" || fail "changed: $out"
echo "$out" | grep -q "renamed fn helper to fn twice" || fail "renamed: $out"
echo "ok moved, changed and renamed"
t commit -am "move and rename" > /dev/null
h=$(t history geo.twice)
echo "$h" | grep -q "renamed fn helper to fn twice" || fail "history of twice: $h"
echo "$h" | grep -q "added fn helper" || fail "history did not follow the rename: $h"
h=$(t history geo.Area)
echo "$h" | grep -q "moved fn Area" || fail "history of Area: $h"
echo "$h" | grep -q "added fn Area in geo/a.tin" || fail "history did not follow the move: $h"
echo "$h" | grep -q "reformat" && fail "history listed the reformat: $h"
echo "ok history follows renames and moves"

# the same on the real compiler: a function (its doc comment with it) moves from parse.tin to lower.tin while
# check.tin is reformatted (spacing tin fmt would undo); the moved function is the only change
mkdir -p "$d/real/compiler"
cd "$d/real"
t init . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
for f in parse lower check; do
	cp "$root/toolchain/compiler/$f.tin" compiler/
done
t add .
t commit -m "the compiler" > /dev/null
name=$(grep '^fn [a-z]' compiler/parse.tin | awk 'NR == 40 { print $2 }' | sed 's/(.*//')
[ -n "$name" ] || fail "no function in parse.tin"
perl -0ne "print \$1 if /\n\n((?:\/\/[^\n]*\n)*fn $name\(.*?\n}\n)/s" compiler/parse.tin > "$d/moved.tin"
[ -s "$d/moved.tin" ] || fail "cannot cut $name out of parse.tin"
perl -0pi -e "s/\n\n(?:\/\/[^\n]*\n)*fn $name\(.*?\n}\n/\n/s" compiler/parse.tin
printf '\n' >> compiler/lower.tin
cat "$d/moved.tin" >> compiler/lower.tin
perl -pi -e 's/$/  / if /^\t/' compiler/check.tin
out=$(t diff --semantic)
[ "$out" = "$(printf 'compiler/lower.tin\n  moved fn %s (from compiler/parse.tin)' "$name")" ] || fail "the real move: $out"
echo "ok on the real compiler, $name moved from parse.tin to lower.tin, and check.tin reformatted is no change"
