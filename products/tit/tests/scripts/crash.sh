#!/bin/sh
# The crash harness (#806): every command that changes the repository is run once for every crash point it reaches
# (TIT_CRASH_AT=n ends the process at its n-th durable write, status 86); after each crash, the next tit command
# recovers, and the refs and HEAD must then be exactly as before the command or exactly as after it, with every
# branch readable. Usage: crash.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d" XDG_CONFIG_HOME="$d" TIT_NO_PAGER=1 TIT_TIME=1700000000
export GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=Ada GIT_AUTHOR_EMAIL=ada@example.com GIT_COMMITTER_NAME=Ada GIT_COMMITTER_EMAIL=ada@example.com
fail() {
	echo "FAIL crash: $*"
	exit 1
}
t() { "$tit" "$@"; }
# snapshot: every ref and HEAD
snap() {
	(
		cd "$1"
		find .tit/refs -type f | sort | while read f; do echo "$f $(cat "$f")"; done
		cat .tit/workspaces/default/HEAD
	)
}
readable() {
	(
		cd "$1"
		for b in $(find .tit/refs/heads -type f | sed 's|.tit/refs/heads/||'); do
			t log --format=id "$b" > /dev/null || exit 1
		done
	)
}
total=0
# check <name> <template dir> <command...>: runs the command at every crash point on copies of the template
check() {
	name=$1
	tpl=$2
	shift 2
	rm -rf "$d/after"
	cp -R "$tpl" "$d/after"
	(cd "$d/after" && "$@" > /dev/null 2>&1) || fail "$name does not run without a crash"
	before=$(snap "$tpl")
	after=$(snap "$d/after")
	[ "$before" != "$after" ] || fail "$name changes no ref"
	n=1
	while :; do
		rm -rf "$d/w"
		cp -R "$tpl" "$d/w"
		set +e
		(cd "$d/w" && TIT_CRASH_AT=$n "$@" > /dev/null 2>&1)
		rc=$?
		set -e
		if [ $rc -ne 86 ]; then
			[ $rc -eq 0 ] || fail "$name with TIT_CRASH_AT=$n failed with status $rc"
			break
		fi
		(cd "$d/w" && t status -s > /dev/null 2>&1) || fail "$name, crash $n: the next command fails"
		now=$(snap "$d/w")
		[ "$now" = "$before" ] || [ "$now" = "$after" ] || fail "$name, crash $n: the repository is neither before nor after:
$now"
		readable "$d/w" || fail "$name, crash $n: a branch does not read"
		[ ! -e "$d/w/.tit/oplog/pending.op" ] || fail "$name, crash $n: an operation is still pending"
		n=$((n + 1))
	done
	total=$((total + n - 1))
	echo "ok $name: $((n - 1)) crash points, each before or after"
}

mkdir "$d/base"
cd "$d/base"
t init . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
printf 'one\n' > a.txt
printf 'b\n' > b.txt
t add a.txt b.txt
t commit -m first > /dev/null
t branch side > /dev/null
t switch side > /dev/null
printf 'side\n' > s.txt
t add s.txt
t commit -m side > /dev/null
t switch main > /dev/null
printf 'two\n' >> a.txt
t add a.txt

check "commit" "$d/base" "$tit" commit -m second
check "commit --amend" "$d/base" "$tit" commit --amend -m "first, amended"
rm -rf "$d/clean" && cp -R "$d/base" "$d/clean" && (cd "$d/clean" && t commit -m second > /dev/null)
check "merge" "$d/clean" "$tit" merge side
check "switch" "$d/clean" "$tit" switch side
check "branch" "$d/clean" "$tit" branch other
check "tag -a" "$d/clean" "$tit" tag -a v1 -m one
check "undo" "$d/clean" "$tit" undo
rm -rf "$d/stk" && cp -R "$d/clean" "$d/stk" && (cd "$d/stk" && t switch -c feat > /dev/null && printf 'x\n' > x.txt && t add x.txt && t commit -m x > /dev/null && printf 'y\n' > y.txt && t add y.txt && t commit -m y > /dev/null)
x=$(cd "$d/stk" && t log --format=change -n 1 HEAD~1)
y=$(cd "$d/stk" && t log --format=change -n 1)
check "move" "$d/stk" "$tit" move "$y" --before "$x"
# adopt, the second time: only the new git commit
mkdir "$d/g"
cd "$d/g"
git init -q -b main .
printf 'g\n' > g.txt
git add g.txt && git commit -q -m g1
t adopt > /dev/null
printf 'g2\n' >> g.txt && git commit -q -am g2
check "adopt" "$d/g" "$tit" adopt
echo "ok $total crash points in all"
