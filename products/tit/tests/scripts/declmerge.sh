#!/bin/sh
# Merge by declaration and overlap (#799): two branches adding different functions at the same place merge cleanly
# (git's line merge conflicts there); one function both changed conflicts and is named; overlap names the branch
# that changes the same declaration. Usage: declmerge.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d" XDG_CONFIG_HOME="$d" TIT_NO_PAGER=1
fail() {
	echo "FAIL declmerge: $*"
	exit 1
}
t() { "$tit" "$@"; }
root=$PWD
mkdir -p "$d/r/compiler"
cd "$d/r"
t init . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
cat > compiler/check.tin <<'X'
package compiler

fn checkExpr(e i64) i64 {
	return e
}

fn lowerCall(c i64) i64 {
	return c + 1
}
X
t add .
t commit -m base > /dev/null
t switch -c left > /dev/null
perl -0pi -e 's/(fn lowerCall)/fn checkLeft(x i64) i64 {\n\treturn x * 2\n}\n\n$1/' compiler/check.tin
t commit -am "left adds checkLeft" > /dev/null
t switch main > /dev/null
perl -0pi -e 's/(fn lowerCall)/fn checkRight(x i64) i64 {\n\treturn x * 3\n}\n\n$1/' compiler/check.tin
t commit -am "right adds checkRight" > /dev/null
t merge left > "$d/out.txt" 2>&1 || fail "the merge conflicted: $(cat "$d/out.txt")"
grep -q '^fn checkLeft' compiler/check.tin && grep -q '^fn checkRight' compiler/check.tin || fail "a function is missing: $(cat compiler/check.tin)"
[ "$(t status -s)" = "" ] || fail "status after the merge: $(t status -s)"
echo "ok two functions added at the same place merge by declaration"

t switch -c a > /dev/null
perl -pi -e 's/return c \+ 1/return c + 10/' compiler/check.tin
t commit -am "a changes lowerCall" > /dev/null
t switch main > /dev/null
t switch -c b > /dev/null
perl -pi -e 's/return c \+ 1/return c + 20/' compiler/check.tin
t commit -am "b changes lowerCall" > /dev/null
t overlap > "$d/overlap.txt"
grep -q '^a also changes:' "$d/overlap.txt" || fail "overlap: $(cat "$d/overlap.txt")"
grep -q 'compiler: fn lowerCall' "$d/overlap.txt" || fail "overlap did not name lowerCall: $(cat "$d/overlap.txt")"
echo "ok overlap names lowerCall and the other branch"
if t merge a > "$d/out.txt" 2>&1; then
	fail "both changed lowerCall and the merge went through"
fi
grep -q 'CONFLICT: compiler/check.tin (fn lowerCall)' "$d/out.txt" || fail "the conflict: $(cat "$d/out.txt")"
echo "ok one function changed on both sides conflicts, named"

# the same on the real check.tin: each branch adds a function above the same one (and its doc comment), half way
# down the file
mkdir -p "$d/real/compiler"
cd "$d/real"
t init . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
cp "$root/toolchain/compiler/check.tin" compiler/check.tin
t add .
t commit -m base > /dev/null
anchor=$(grep '^fn [a-zA-Z]' compiler/check.tin | awk 'NR == 200 { print $2 }' | sed 's/(.*//')
[ -n "$anchor" ] || fail "no anchor in check.tin"
cp compiler/check.tin "$d/real.base"
t switch -c left > /dev/null
perl -0pi -e "s/\n\n((?:\/\/[^\n]*\n)*fn $anchor\\()/\n\nfn checkLeftAdded(x i64) i64 {\n\treturn x * 2\n}\n\n\$1/" compiler/check.tin
t commit -am "left adds a function" > /dev/null
cp compiler/check.tin "$d/real.theirs"
t switch main > /dev/null
perl -0pi -e "s/\n\n((?:\/\/[^\n]*\n)*fn $anchor\\()/\n\nfn checkRightAdded(x i64) i64 {\n\treturn x * 3\n}\n\n\$1/" compiler/check.tin
t commit -am "right adds a function" > /dev/null
cp compiler/check.tin "$d/real.ours"
if git merge-file -p "$d/real.ours" "$d/real.base" "$d/real.theirs" > /dev/null 2>&1; then
	fail "git's line merge did not conflict on check.tin; the case tests nothing"
fi
t merge left > "$d/out.txt" 2>&1 || fail "the merge of the real check.tin conflicted: $(cat "$d/out.txt")"
[ "$(grep -c '^fn check\(Left\|Right\)Added' compiler/check.tin)" = 2 ] || fail "a function is missing from the real check.tin"
perl -0pe "s/fn check(Left|Right)Added[^}]*}\n\n//g" compiler/check.tin | cmp -s - "$d/real.base" || fail "the merged check.tin is not the base with the two functions added"
echo "ok on the real check.tin, two functions added before $anchor merge where git's line merge conflicts"
