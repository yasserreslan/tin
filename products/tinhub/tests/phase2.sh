#!/bin/sh
# Phase 1 to phase 2 (#1026, #1015) against TINHUB_TEST_DB, the fake S3 at TINHUB_TEST_S3 and Redis.
# Usage: phase2.sh TINHUB REPO_DRIVER TIT
#   - a phase 1 node (packs on disk) has a repository; tinhub packs copy fills the bucket and the same database serves
#     on: config only (packs.store = s3) turns it into phase 2, two stateless nodes and a worker node;
#   - both nodes serve the same clone; tinhub check finds every live pack in the bucket;
#   - a node killed (SIGKILL) mid-push, mid-fetch and the worker node killed mid-job: every push tit reported is on
#     main, read through the other node, no live pack is missing, and every job finishes.
set -eu
hub=$1
driver=$2
tit=$3
tmp=$(mktemp -d)
pa=
pb=
pw=
. "$(dirname "$0")/lib.sh"
cleanup() {
	for p in $pa $pb $pw; do kill -9 "$p" 2>/dev/null || true; done
	stop_redis
	rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
fail() {
	echo "FAIL tinhub phase 2: $*"
	for n in a b w; do [ -f "$tmp/$n.log" ] && tail -n 5 "$tmp/$n.log"; done
	exit 1
}
[ -n "${TINHUB_TEST_S3:-}" ] || { echo "SKIP tinhub phase 2 (TINHUB_TEST_S3 is not set)"; exit 0; }
start_redis || fail "Redis is needed"
"$driver" setup "$tmp/p1" > /dev/null
dbaddr=${TINHUB_TEST_DB%%/*}
export TINHUB_DB_ADDR="$dbaddr" TINHUB_DB_NAME="${TINHUB_TEST_DB#*/}" TINHUB_DB_USER="${TINHUB_TEST_DB_USER:-}" TINHUB_DB_PASSWORD="${TINHUB_TEST_DB_PASSWORD:-}"
export TINHUB_SECRETS_NONCE=test-nonce-secret-0123456789 TINHUB_SECRETS_COOKIE=test-cookie-key-0123456789
export TINHUB_PACKS_SWEEP=0 TINHUB_REDIS_ADDR="127.0.0.1:$port" TINHUB_WORKERS_LEASE=3s TIN_CORES=2
export TINHUB_S3_ENDPOINT="$TINHUB_TEST_S3" TINHUB_S3_ACCESS_KEY="$TINHUB_TEST_S3_KEY" TINHUB_S3_SECRET_KEY="$TINHUB_TEST_S3_SECRET"
export TINHUB_S3_REGION=us-east-1 TINHUB_S3_BUCKET="tinhub-phase2-$$" TIT_NO_PAGER=1
a=http://127.0.0.1:18441
b=http://127.0.0.1:18442
# node NAME ADDR ROLES DIR: a node in the background, its pid in $last
node() {
	TINHUB_LISTEN=$2 TINHUB_ROLES=$3 TINHUB_PACKS_DIR=$4 "$hub" run >> "$tmp/$1.log" 2>&1 &
	last=$!
	for i in $(seq 1 40); do
		curl -sf "http://$2/healthz" > /dev/null 2>&1 && return 0
		sleep 0.25
	done
	fail "node $1 did not start"
}

# phase 1: packs on disk; the first clone
TINHUB_PACKS_STORE=dir node a 127.0.0.1:18441 node,worker "$tmp/p1"
pa=$last
(cd "$tmp" && "$tit" clone "$a/ada/tin" one > "$tmp/out" 2>&1) || fail "the phase 1 clone: $(cat "$tmp/out")"
kill -TERM "$pa"
wait "$pa" || true
pa=

# the move: tinhub packs copy, then config only
TINHUB_PACKS_STORE=dir TINHUB_PACKS_DIR="$tmp/p1" "$hub" packs copy > "$tmp/copy.out" 2>&1 || fail "packs copy: $(cat "$tmp/copy.out")"
grep -q 'copied [1-9]' "$tmp/copy.out" || fail "packs copy copied nothing: $(cat "$tmp/copy.out")"
export TINHUB_PACKS_STORE=s3
TINHUB_PACKS_DIR="$tmp/check" "$hub" check > "$tmp/check.out" || fail "tinhub check on the bucket: $(cat "$tmp/check.out")"
node a 127.0.0.1:18441 node "$tmp/na"
pa=$last
node b 127.0.0.1:18442 node "$tmp/nb"
pb=$last
node w 127.0.0.1:18443 worker "$tmp/nw"
pw=$last
for n in a b; do
	url=$(eval echo "\$$n")
	(cd "$tmp" && "$tit" clone "$url/ada/tin" "two$n" > "$tmp/out" 2>&1) || fail "the phase 2 clone from $n: $(cat "$tmp/out")"
	[ "$(cat "$tmp/two$n/a.txt")" = "$(cat "$tmp/one/a.txt")" ] || fail "node $n serves another tree"
done
echo "PASS tinhub phase 2: packs copy and config move phase 1 to two nodes on a bucket"

# a writer: bob, with write on ada/tin
bob() {
	HOME="$tmp/bob" XDG_CONFIG_HOME="$tmp/bob" "$tit" "$@"
}
mkdir -p "$tmp/bob"
bob config set --user user.name Bob
bob config set --user user.email bob@example.com
"$hub" admin invite bob@example.com > "$tmp/invite.out"
code=$(sed -n 's/^invite for bob@example.com: \(.*\)$/\1/p' "$tmp/invite.out")
bob key add "$a" "$code" > "$tmp/out" 2>&1 || fail "bob's key: $(cat "$tmp/out")"
"$driver" grant bob@example.com write
(cd "$tmp" && bob clone "$a/ada/tin" work > "$tmp/out" 2>&1) || fail "bob's clone: $(cat "$tmp/out")"
bob -C "$tmp/work" remote add b "$b/ada/tin" > /dev/null

# chaos: pushes through node a while it is killed and restarted, each push retried on node b when a is down
(
	cd "$tmp/work"
	for i in $(seq 1 15); do
		head -c 200000 /dev/urandom > "big$i.bin"
		printf '%s\n' "$i" > "n$i.txt"
		bob add "big$i.bin" "n$i.txt"
		bob commit -m "chaos $i" > /dev/null
		n=0
		until bob push > "$tmp/push.out" 2>&1 || bob push b > "$tmp/push.out" 2>&1; do
			n=$((n + 1))
			[ $n -lt 100 ] || { echo "push $i: $(cat "$tmp/push.out")"; exit 1; }
			bob pull b > /dev/null 2>&1 || true
			sleep 0.1
		done
	done
) > "$tmp/load.out" 2>&1 &
load=$!
# fetches through node a too, cut off by the same kills
(
	for i in $(seq 1 30); do
		rm -rf "$tmp/f"
		(cd "$tmp" && "$tit" clone "$a/ada/tin" f > /dev/null 2>&1) || true
	done
) &
fetches=$!
for r in 1 2 3; do
	sleep 0.4
	kill -9 "$pa"
	wait "$pa" 2>/dev/null || true
	kill -9 "$pw"
	wait "$pw" 2>/dev/null || true
	node a 127.0.0.1:18441 node "$tmp/na"
	pa=$last
	node w 127.0.0.1:18443 worker "$tmp/nw"
	pw=$last
done
wait $load || fail "the pushes under kills: $(cat "$tmp/load.out")"
wait $fetches || true
rm -rf "$tmp/after"
(cd "$tmp" && bob clone "$b/ada/tin" after > "$tmp/out" 2>&1) || fail "the clone after the kills: $(cat "$tmp/out")"
for i in $(seq 1 15); do
	[ "$(cat "$tmp/after/n$i.txt" 2>/dev/null)" = "$i" ] || fail "push $i is not on main after the kills"
	cmp -s "$tmp/after/big$i.bin" "$tmp/work/big$i.bin" || fail "push $i's file differs after the kills"
done
TINHUB_PACKS_DIR="$tmp/check" "$hub" check > "$tmp/check.out" || fail "a live pack is missing after the kills: $(cat "$tmp/check.out")"
# every job finishes: the worker node reclaims what the killed one held once its lease (3s) runs out (the nightly
# capsule retention waits for its time, so it is not counted)
done=0
for i in $(seq 1 120); do
	m=$(curl -sf "http://127.0.0.1:18443/metrics") || m=
	ready=$(echo "$m" | awk '/^tinhub_jobs_ready/ && !/replay.retention/ { s += $2 } END { print s + 0 }')
	claimed=$(echo "$m" | awk '/^tinhub_jobs_claimed/ { print $2 + 0 }')
	dead=$(echo "$m" | awk '/^tinhub_jobs_dead/ { s += $2 } END { print s + 0 }')
	if [ -n "$m" ] && [ "$ready" = 0 ] && [ "$claimed" = 0 ]; then done=1; break; fi
	sleep 0.5
done
[ $done = 1 ] || fail "jobs left: $m"
[ "$dead" = 0 ] || fail "dead jobs after the kills: $m"
echo "PASS tinhub phase 2 chaos: nodes killed mid-push, mid-fetch and mid-job lose no push, no pack and no job"
