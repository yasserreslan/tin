#!/bin/sh
# tit bisect --run and tit verify (#801): in a 1,000-commit history where one change breaks a check, bisect names
# that change in at most 11 runs and puts the working directory back; verify reports unsigned commits without
# failing. Usage: bisect.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d" XDG_CONFIG_HOME="$d" TIT_NO_PAGER=1
fail() {
	echo "FAIL bisect: $*"
	exit 1
}
t() { "$tit" "$@"; }
mkdir "$d/r"
cd "$d/r"
t init . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com
printf '0\n' > n.txt
printf 'ok\n' > state.txt
t add n.txt state.txt
t commit -m "commit 0" > /dev/null
i=1
while [ $i -lt 1000 ]; do
	printf '%s\n' $i > n.txt
	if [ $i -eq 637 ]; then
		printf 'broken\n' > state.txt
	fi
	t commit -am "commit $i" > /dev/null
	i=$((i + 1))
done
t bisect --run 'grep -qx ok state.txt' --good HEAD~999 > "$d/out.txt" 2> "$d/log.txt" || fail "bisect: $(cat "$d/out.txt" "$d/log.txt")"
grep -q "commit 637$" "$d/out.txt" || fail "bisect named another change: $(cat "$d/out.txt")"
runs=$(sed -n 's/.*found in \([0-9]*\) runs.*/\1/p' "$d/out.txt")
[ "$runs" -le 11 ] || fail "$runs runs"
[ "$(cat n.txt)" = "999" ] || fail "the working directory was not put back"
[ "$(t status -s)" = "" ] || fail "status after bisect: $(t status -s)"
echo "ok bisect found commit 637 in $runs runs"
t verify HEAD~2..HEAD > "$d/verify.txt" || fail "verify: $(cat "$d/verify.txt")"
[ "$(grep -c 'unsigned, no signers file' "$d/verify.txt")" = 2 ] || fail "verify said: $(cat "$d/verify.txt")"
echo "ok verify"
