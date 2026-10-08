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
