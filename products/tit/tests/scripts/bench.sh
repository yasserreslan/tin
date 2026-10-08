#!/bin/sh
# tit bench (#802): a result follows its change through an amend; two results from one machine compare; a macOS
# result never compares with a Linux one, nor results from two machines. Usage: bench.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d" XDG_CONFIG_HOME="$d" TIT_NO_PAGER=1
fail() {
	echo "FAIL bench: $*"
	exit 1
}
t() { "$tit" "$@"; }
mkdir "$d/r"
cd "$d/r"
t init . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
printf 'one\n' > f.txt
t add f.txt
t commit -m one > /dev/null
t bench record startup 12.5 --unit ms > /dev/null
printf 'one, better\n' > f.txt
t commit -a --amend > /dev/null
t bench log startup | grep -q "^$(t log --format=change -n 1 | cut -c1-12) .* 12.5 ms" || fail "the result did not follow the amend: $(t bench log startup)"
echo "ok a result follows its change"
printf 'two\n' >> f.txt
t commit -am two > /dev/null
t bench record startup 10 --unit ms > /dev/null
t bench compare startup HEAD~1 HEAD | grep -q "^startup: 12.5 -> 10 ms, ratio 0.8" || fail "compare: $(t bench compare startup HEAD~1 HEAD)"
echo "ok one machine's results compare"
# a result from the other system, and one from another machine of this system
here=$(uname -s)
other=Linux
[ "$here" = Linux ] && other=Darwin
change=$(t log --format=change -n 1)
commit=$(t log --format=id -n 1)
f=.tit/bench/startup.jsonl
sed -n '$p' $f | sed "s/\"Os\":\"$here\"/\"Os\":\"$other\"/; s/\"Value\":\"10\"/\"Value\":\"9\"/" > other.line
cat other.line >> $f
if t bench compare startup HEAD~1 HEAD > "$d/out.txt" 2>&1; then
	fail "macOS was compared with Linux"
fi
grep -q "never compared with Linux" "$d/out.txt" || fail "the refusal: $(cat "$d/out.txt")"
sed '$d' $f > x && mv x $f
sed -n '$p' $f | sed 's/"Cores":"[0-9]*"/"Cores":"999"/' >> $f
if t bench compare startup HEAD~1 HEAD > "$d/out.txt" 2>&1; then
	fail "two machines were compared"
fi
grep -q "two machines" "$d/out.txt" || fail "the refusal: $(cat "$d/out.txt")"
echo "ok macOS against Linux, and two machines, are refused"
