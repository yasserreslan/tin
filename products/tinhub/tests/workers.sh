#!/bin/sh
# The repository workers' crash tests (#1014) against TINHUB_TEST_DB. Usage: workers.sh WORKERS_DRIVER
# A repack, a prune and a purge, each killed at every crash point in turn (TIT_CRASH_AT), leave every ref whole; run
# again, each finishes its work, and the sweep then leaves the pack rows and the store's files agreeing.
set -eu
driver=$1
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

for job in repack prune purge; do
	n=1
	while :; do
		"$driver" setup "$tmp/node" > /dev/null
		status=0
		TIT_CRASH_AT=$n "$driver" run "$tmp/node" $job > "$tmp/crash.out" 2>&1 || status=$?
		if [ $status != 0 ] && [ $status != 86 ]; then
			cat "$tmp/crash.out"; echo "FAIL tinhub $job crash at point $n: status $status"; exit 1
		fi
		"$driver" check "$tmp/node" $job > "$tmp/check.out" 2>&1 || { cat "$tmp/check.out"; echo "FAIL tinhub $job crash at point $n"; exit 1; }
		[ $status = 86 ] || break
		n=$((n + 1))
		[ $n -le 100 ] || { echo "FAIL tinhub $job crash: more than 100 crash points"; exit 1; }
	done
	echo "PASS tinhub $job crash points: $((n - 1)) crashes, each left the repository whole and a rerun finished it"
done

# a mirror into a local bare git directory: git clones it, finds main's tree, and fsck finds nothing wrong
if command -v git > /dev/null 2>&1; then
	"$driver" setup "$tmp/node" > /dev/null
	git init -q --bare "$tmp/mirror.git"
	"$driver" mirror "$tmp/node" "$tmp/mirror.git" > "$tmp/mirror.out" 2>&1 || { cat "$tmp/mirror.out"; echo "FAIL tinhub mirror"; exit 1; }
	git --git-dir "$tmp/mirror.git" fsck --strict > "$tmp/fsck.out" 2>&1 || { cat "$tmp/fsck.out"; echo "FAIL tinhub mirror: git fsck"; exit 1; }
	git clone -q "$tmp/mirror.git" "$tmp/clone" 2> "$tmp/clone.err" || { cat "$tmp/clone.err"; echo "FAIL tinhub mirror: git clone"; exit 1; }
	[ "$(cat "$tmp/clone/a.txt")" = "version 3" ] || { echo "FAIL tinhub mirror: the clone's a.txt is not main's"; exit 1; }
	[ "$(git -C "$tmp/clone" rev-list --count HEAD)" = 4 ] || { echo "FAIL tinhub mirror: main's history"; exit 1; }
	git -C "$tmp/clone" log -1 --format=%B | grep -q '^Change-Id: ' || { echo "FAIL tinhub mirror: no Change-Id line"; exit 1; }
	"$driver" mirror "$tmp/node" "$tmp/mirror.git" | grep -q '^mirrored: 0 objects' || { echo "FAIL tinhub mirror: a second run wrote objects"; exit 1; }
	echo "PASS tinhub mirror: git clones the mirror, with main's history"
else
	echo "SKIP tinhub mirror (no git)"
fi
