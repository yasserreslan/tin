#!/bin/sh
# Status on one core (#807): tit status runs on core 0 alone (starting the other cores costs more than a warm status
# takes), and commit -a hashes on every core. 600 files, 300 of them changed: status lists exactly those, as it does
# with TIN_CORES=1, and after commit -a it is clean. Usage: status_many.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1
fail() {
	echo "FAIL status_many: $*"
	exit 1
}
t() { "$tit" "$@"; }
mkdir -p "$HOME" "$d/r"
cd "$d/r"
t init . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
i=0
while [ $i -lt 600 ]; do
	printf 'file %d\n' $i > "f$i.txt"
	i=$((i + 1))
done
t add . > /dev/null
t commit -m "600 files" > /dev/null
# change every other file, to the same size, a second later (the stat cache must not trust them)
sleep 1
i=0
while [ $i -lt 600 ]; do
	printf 'FILE %d\n' $i > "f$i.txt"
	i=$((i + 2))
done
n=$(t status -s | grep -c '^ *M')
[ "$n" = 300 ] || fail "status listed $n changed files, not 300: $(t status -s | head -5)"
n1=$(TIN_CORES=1 "$tit" status -s | grep -c '^ *M')
[ "$n1" = 300 ] || fail "status on one core listed $n1 changed files, not 300"
echo "ok status lists the 300 changed files of 600"
t commit -am "300 changed" > /dev/null
[ "$(t status -s)" = "" ] || fail "status after the commit: $(t status -s | head -5)"
echo "ok status is clean after the commit"
