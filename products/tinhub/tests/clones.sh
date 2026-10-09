#!/bin/sh
# Clone timings and node scaling (#1015, #1018, #1026) against TINHUB_TEST_DB, the S3 at TINHUB_TEST_S3 and Redis.
# Usage: clones.sh TINHUB REPO_DRIVER TIT FETCH_LOAD [SOURCE]
#   - SOURCE (a git repository, the Tin repo by default) is mirrored into tinhub with deploy/mirror-sync.sh, cloned
#     with tit and pushed to: the Tin repo on tinhub;
#   - clone times on the directory store, then from the bucket with a cold node cache (a fresh node) and a warm one;
#   - full fetches (a clone without tit's checkout, programs/fetch_load.tin) finished in TINHUB_BENCH_SECONDS (20) by
#     TINHUB_BENCH_CLIENTS processes (6) against one node and spread over three, each node on one core (TIN_CORES=1):
#     the ratio, how busy each node's core was, and fetches per node core-second. A tit clone spends most of its time in
#     the client (the checkout), so tit clones would measure that. When the clients share the machine and the nodes'
#     cores are not all busy, the ratio is the machine's limit; equal fetches per core-second mean the nodes scale.
# Numbers are medians and ratios from one Linux machine; it prints them and checks only that every clone succeeded.
set -eu
hub=$1
driver=$2
tit=$3
fetchload=$4
src=${5:-$(cd "$(dirname "$0")/../../.." && pwd)}
runs=${TINHUB_BENCH_RUNS:-5}
seconds=${TINHUB_BENCH_SECONDS:-20}
clients=${TINHUB_BENCH_CLIENTS:-6}
tmp=$(mktemp -d)
deploy=$(cd "$(dirname "$0")/../deploy" && pwd)
pids=
. "$(dirname "$0")/lib.sh"
cleanup() {
	for p in $pids; do kill -9 "$p" 2>/dev/null || true; done
	stop_redis
	rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
fail() {
	echo "FAIL tinhub clones: $*"
	for f in "$tmp"/*.log; do [ -f "$f" ] && tail -n 5 "$f"; done
	exit 1
}
[ -n "${TINHUB_TEST_S3:-}" ] || { echo "SKIP tinhub clones (TINHUB_TEST_S3 is not set)"; exit 0; }
start_redis || fail "Redis is needed"
"$driver" setup "$tmp/p1" > /dev/null
"$driver" repo tinsrc
dbaddr=${TINHUB_TEST_DB%%/*}
export TINHUB_DB_ADDR="$dbaddr" TINHUB_DB_NAME="${TINHUB_TEST_DB#*/}" TINHUB_DB_USER="${TINHUB_TEST_DB_USER:-}" TINHUB_DB_PASSWORD="${TINHUB_TEST_DB_PASSWORD:-}"
export TINHUB_SECRETS_NONCE=test-nonce-secret-0123456789 TINHUB_SECRETS_COOKIE=test-cookie-key-0123456789
export TINHUB_PACKS_SWEEP=0 TINHUB_REDIS_ADDR="127.0.0.1:$port"
export TINHUB_S3_ENDPOINT="$TINHUB_TEST_S3" TINHUB_S3_ACCESS_KEY="$TINHUB_TEST_S3_KEY" TINHUB_S3_SECRET_KEY="$TINHUB_TEST_S3_SECRET"
export TINHUB_S3_REGION=us-east-1 TINHUB_S3_BUCKET="tinhub-clones-$$" TIT_NO_PAGER=1

# node NAME PORT STORE DIR [CORES]: a node in the background, its pid in $last
node() {
	TINHUB_LISTEN=127.0.0.1:$2 TINHUB_PUBLIC_URL=http://127.0.0.1:$2 TINHUB_ROLES=node TINHUB_PACKS_STORE=$3 TINHUB_PACKS_DIR=$4 TIN_CORES=${5:-2} "$hub" run >> "$tmp/$1.log" 2>&1 &
	last=$!
	pids="$pids $last"
	for i in $(seq 1 40); do
		curl -sf "http://127.0.0.1:$2/healthz" > /dev/null 2>&1 && return 0
		sleep 0.25
	done
	fail "node $1 did not start"
}
stop() {
	kill -TERM "$1" 2>/dev/null || true
	wait "$1" 2>/dev/null || true
}
# ms: milliseconds since the epoch
ms() {
	date +%s%3N
}
# median of the numbers on stdin
median() {
	sort -n | awk '{ a[NR] = $1 } END { if (NR % 2) print a[(NR + 1) / 2]; else print int((a[NR / 2] + a[NR / 2 + 1]) / 2) }'
}
# clone URL DIR: one clone, its time in ms on stdout
clone() {
	rm -rf "$2"
	t0=$(ms)
	(cd "$tmp" && "$tit" clone "$1" "$2" > "$2.out" 2>&1) || fail "clone $1: $(cat "$2.out")"
	echo $(($(ms) - t0))
}
kit() {
	HOME="$tmp/$1" XDG_CONFIG_HOME="$tmp/$1" "$tit" "$@"
}
bot() {
	HOME="$tmp/bot" XDG_CONFIG_HOME="$tmp/bot" "$tit" "$@"
}

# the Tin repo on tinhub: mirrored from git, cloned, pushed to
node a 18451 dir "$tmp/p1"
pa=$last
base=http://127.0.0.1:18451
mkdir -p "$tmp/bot"
bot config set --user user.name Mirror
bot config set --user user.email mirror@example.com
"$hub" admin invite mirror@example.com > "$tmp/invite.out" || fail "the mirror's invite"
code=$(sed -n 's/^invite for mirror@example.com: \(.*\)$/\1/p' "$tmp/invite.out")
bot key add "$base" "$code" > "$tmp/out" 2>&1 || fail "the mirror's key: $(cat "$tmp/out")"
"$driver" grant mirror@example.com write tinsrc
if [ "$(git -C "$src" rev-parse --is-shallow-repository)" = true ]; then
	# a shallow checkout (CI's) has no history to adopt: its tree at HEAD as one commit
	git init -q -b main "$tmp/origin"
	git -C "$src" archive HEAD | tar -x -C "$tmp/origin"
	git -C "$tmp/origin" add -A
	GIT_AUTHOR_NAME=Ada GIT_AUTHOR_EMAIL=ada@example.com GIT_COMMITTER_NAME=Ada GIT_COMMITTER_EMAIL=ada@example.com git -C "$tmp/origin" commit -q -m "the tree at $(git -C "$src" rev-parse --short HEAD)"
	src=$tmp/origin
fi
git clone -q --no-local "$src" "$tmp/mirror"
branch=$(git -C "$tmp/mirror" rev-parse --abbrev-ref HEAD)
(cd "$tmp/mirror" && bot adopt > /dev/null 2>&1 && bot remote add tinhub "$base/ada/tinsrc") || fail "adopting $src"
t0=$(ms)
HOME="$tmp/bot" XDG_CONFIG_HOME="$tmp/bot" TIT="$tit" sh "$deploy/mirror-sync.sh" "$tmp/mirror" "$branch" > "$tmp/out" 2>&1 || fail "mirror-sync: $(cat "$tmp/out")"
mirrorms=$(($(ms) - t0))
files=$(git -C "$tmp/mirror" ls-files | wc -l)
commits=$(git -C "$tmp/mirror" rev-list --count HEAD)
first=$(clone "$base/ada/tinsrc" first)
[ "$(cd "$tmp/first" && find . -type f ! -path './.tit/*' | wc -l)" -eq "$files" ] || fail "the clone has another number of files than $files"
cmp -s "$tmp/first/README.md" "$tmp/mirror/README.md" || fail "the clone's README.md differs"
cd "$tmp/first"
bot config set user.email mirror@example.com > /dev/null 2>&1 || true
printf 'pushed through tinhub\n' > tinhub-push.txt
bot add tinhub-push.txt
bot commit -m "a push through tinhub" > /dev/null
bot push > "$tmp/out" 2>&1 || fail "the push: $(cat "$tmp/out")"
cd "$tmp"
again=$(clone "$base/ada/tinsrc" again)
[ -f "$tmp/again/tinhub-push.txt" ] || fail "the pushed file is not in a new clone"
echo "PASS tinhub clones: the Tin repo ($commits commits, $files files) mirrored in ${mirrorms} ms, cloned and pushed to"

# phase 1: the directory store
cpu0=$(awk '{ print $14 + $15 }' "/proc/$pa/stat")
t0=$(ms)
for i in $(seq 1 "$runs"); do clone "$base/ada/tinsrc" "d$i"; done | median > "$tmp/dir.ms"
cpu=$(( ($(awk '{ print $14 + $15 }' "/proc/$pa/stat") - cpu0) * 1000 / $(getconf CLK_TCK) ))
echo "tinhub: the server's CPU for $runs clones: $cpu ms of $(($(ms) - t0)) ms"
stop "$pa"

# phase 2: packs copy into the bucket; a fresh node has a cold cache, its second clone a warm one
TINHUB_PACKS_STORE=dir TINHUB_PACKS_DIR="$tmp/p1" "$hub" packs copy > "$tmp/copy.out" 2>&1 || fail "packs copy: $(cat "$tmp/copy.out")"
: > "$tmp/cold"
: > "$tmp/warm"
for i in $(seq 1 "$runs"); do
	node c 18452 s3 "$tmp/c$i"
	clone http://127.0.0.1:18452/ada/tinsrc "c$i" >> "$tmp/cold"
	clone http://127.0.0.1:18452/ada/tinsrc "w$i" >> "$tmp/warm"
	stop "$last"
done
echo "tinhub clone of the Tin repo, median of $runs: directory store $(cat "$tmp/dir.ms") ms, bucket cold cache $(median < "$tmp/cold") ms, warm cache $(median < "$tmp/warm") ms"

# scaling: CLIENTS fetch_load processes for SECONDS against one node, then spread over three; each node on one core
# cpu PIDS...: the CPU time of these processes so far, in ms
cpu() {
	t=0
	for p in "$@"; do t=$((t + $(awk '{ print $14 + $15 }' "/proc/$p/stat"))); done
	echo $((t * 1000 / $(getconf CLK_TCK)))
}
# load PORTS...: fetches finished by all clients, client i using PORTS[i mod n]
load() {
	n=$#
	i=0
	lp=
	while [ $i -lt "$clients" ]; do
		k=$((i % n + 1))
		eval "p=\${$k}"
		mkdir -p "$tmp/l$i"
		"$fetchload" "http://127.0.0.1:$p/ada/tinsrc" "$tmp/l$i" "$seconds" > "$tmp/l$i.count" 2>&1 &
		lp="$lp $!"
		i=$((i + 1))
	done
	for p in $lp; do wait "$p" || true; done
	awk '/^[0-9]+$/ { s += $1; next } { bad = 1 } END { if (bad) print "fail"; else print s }' "$tmp"/l*.count
}
node one 18453 s3 "$tmp/one" 1
p1=$last
clone http://127.0.0.1:18453/ada/tinsrc warm1 > /dev/null
c0=$(cpu "$p1")
single=$(load 18453)
busy1=$(( ($(cpu "$p1") - c0) / (seconds * 10) ))
stop "$p1"
node x 18454 s3 "$tmp/x" 1
px=$last
node y 18455 s3 "$tmp/y" 1
py=$last
node z 18456 s3 "$tmp/z" 1
pz=$last
for p in 18454 18455 18456; do clone "http://127.0.0.1:$p/ada/tinsrc" "warm$p" > /dev/null; done
c0=$(cpu "$px" "$py" "$pz")
triple=$(load 18454 18455 18456)
busy3=$(( ($(cpu "$px" "$py" "$pz") - c0) / (seconds * 30) ))
stop "$px"
stop "$py"
stop "$pz"
[ "$single" != fail ] && [ "$triple" != fail ] || fail "a fetch under load failed: $(cat "$tmp"/l*.count | tail -n 5)"
echo "tinhub full fetches in ${seconds}s by $clients clients: one node $single, three nodes $triple, ratio $(awk "BEGIN { printf \"%.2f\", $triple / ($single > 0 ? $single : 1) }") (each node's core $busy1% busy alone, $busy3% of three; $(nproc) CPUs shared with the clients)"
echo "tinhub fetches per node core-second: one node $(awk "BEGIN { printf \"%.1f\", $single * 100 / ($seconds * ($busy1 > 0 ? $busy1 : 1)) }"), three nodes $(awk "BEGIN { printf \"%.1f\", $triple * 100 / ($seconds * 3 * ($busy3 > 0 ? $busy3 : 1)) }")"
echo "PASS tinhub clones: timings and scaling measured"
