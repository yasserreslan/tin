#!/bin/sh
# Ctrl-C (#788): an interrupted tit log, in a history long enough to still be running, exits with status 130 and
# leaves nothing behind; so does tit watch, and the next command runs. Usage: interrupt.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d" XDG_CONFIG_HOME="$d" TIT_NO_PAGER=1
fail() {
	echo "FAIL interrupt: $*"
	exit 1
}
t() { "$tit" "$@"; }
# runs a command in the background with SIGINT as a terminal would deliver it (a script's background jobs ignore it)
bg() {
	perl -e '$SIG{INT} = "DEFAULT"; exec @ARGV or exit 127' "$@" &
}
mkdir "$d/r"
cd "$d/r"
git init -q -b main .
perl -e 'for $i (1..8000) { $c = "line $i\n"; $m = "c$i\n"; print "commit refs/heads/main\ncommitter A <a\@b> " . (1700000000 + $i) . " +0000\ndata " . length($m) . "\n$m"; print "M 100644 inline f.txt\ndata " . length($c) . "\n$c\n"; }' | git fast-import --quiet
git checkout -q main
t adopt . > /dev/null
t config set user.name Ada
t config set user.email ada@example.com

bg "$tit" log
pid=$!
sleep 0.05
kill -INT $pid
status=0
wait $pid > /dev/null 2>&1 || status=$?
[ $status = 130 ] || fail "an interrupted log exited with $status"
[ ! -e .tit/lock ] || fail "the interrupted log left the lock"
[ "$(t log -n 1 --format=subject)" = c8000 ] || fail "log after the interrupt"
echo "ok an interrupted log exits with 130"

bg "$tit" watch --every 50 > /dev/null
pid=$!
sleep 0.5
printf 'saved\n' > f.txt
sleep 0.3
kill -INT $pid
status=0
wait $pid > /dev/null 2>&1 || status=$?
[ $status = 130 ] || fail "an interrupted watch exited with $status"
t status -s > /dev/null || fail "status after an interrupted watch"
t timeline f.txt | grep -q "before tit watch" || fail "watch kept nothing before the interrupt"
echo "ok an interrupted watch exits with 130, and the next command runs"
