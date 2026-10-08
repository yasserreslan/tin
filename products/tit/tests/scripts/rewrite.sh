#!/bin/sh
# tit rewrite (#800): one command on every change of five branches; each change's new version is what the command
# left, a failing command is listed and changes nothing, and one undo reverses every branch. Usage: rewrite.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d" XDG_CONFIG_HOME="$d" TIT_NO_PAGER=1
fail() {
	echo "FAIL rewrite: $*"
	exit 1
}
t() { "$tit" "$@"; }
mkdir "$d/r"
cd "$d/r"
t init . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
printf 'base\n' > base.txt
t add base.txt
t commit -m base > /dev/null
for b in b1 b2 b3 b4 b5; do
	t switch -c $b main > /dev/null
	printf 'too   many    spaces in %s\n' $b > $b.txt
	t add $b.txt
	t commit -m "$b first" > /dev/null
	printf 'and   more  here\n' >> $b.txt
	t commit -am "$b second" > /dev/null
done
printf 'x\n' > fail.txt
t add fail.txt
t commit -m "b5 cannot be fixed" > /dev/null
before=""
for b in b1 b2 b3 b4 b5; do before="$before $(t log --format=id -n 1 $b)"; done
cmd='if [ -f fail.txt ]; then echo cannot; exit 3; fi; for f in b*.txt; do perl -pi -e "s/ +/ /g" "$f"; done'
if t rewrite --all "$cmd" > "$d/out.txt" 2>&1; then
	fail "a failing command was not reported"
fi
grep -q "^Rewrote 10 changes on 5 branches\.$" "$d/out.txt" || fail "rewrite said: $(cat "$d/out.txt")"
grep -q "failed: .* b5 cannot be fixed (status 3)" "$d/out.txt" || fail "the failure: $(cat "$d/out.txt")"
for b in b1 b2 b3 b4; do
	t switch $b > /dev/null
	[ "$(cat $b.txt)" = "too many spaces in $b
and more here" ] || fail "$b: $(cat $b.txt)"
	[ "$(t log --format=subject -n 2 | tr '\n' '|')" = "$b second|$b first|" ] || fail "$b lost its changes"
	t diff "HEAD~1" HEAD | grep -q '^+and more here$' || fail "$b second's new version"
done
t switch b5 > /dev/null
[ "$(t log --format=subject -n 1)" = "b5 cannot be fixed" ] || fail "b5's failing change"
echo "ok rewrite --all rewrote every branch and listed the failure"
# the switches above are operations too: undo the rewrite itself, by its number
op=$(t oplog | grep 'tit rewrite' | head -1 | cut -d' ' -f1)
t undo --op "$op" > /dev/null
after=""
for b in b1 b2 b3 b4 b5; do after="$after $(t log --format=id -n 1 $b)"; done
[ "$after" = "$before" ] || fail "one undo did not reverse every branch"
echo "ok one undo reverses it"
