#!/bin/sh
# tinhub's checks (#1005). Usage: run.sh [COMPILER]
# Unit tests run everywhere. With TINHUB_TEST_DB (host:port/name, plus TINHUB_TEST_DB_USER and TINHUB_TEST_DB_PASSWORD)
# the Postgres tests run too: the schema, the event queue's processes, the repository (races, the Redis ref cache,
# crash points), the workers' crash points, the runner's sandboxed replays (Linux) and a server's start, readiness and
# drain.
# The test database is emptied: never point it at a database you need.
set -eu
cd "$(dirname "$0")/../../.." || exit 1
compiler=${1:-bin/tinc}
tmp=$(mktemp -d)
pid=
fake=
cleanup() {
	[ -n "$pid" ] && kill "$pid" 2>/dev/null || true
	[ -n "$fake" ] && kill "$fake" 2>/dev/null || true
	rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
# the S3 pack store's contract runs against bench/ref/s3sig's fake S3 (it checks every signature) when Go is here
if command -v go > /dev/null 2>&1 && (cd bench/ref/s3sig && go build -o "$tmp/s3fake" ./fake) > "$tmp/go.out" 2>&1; then
	"$tmp/s3fake" tinkey 'tin/secret+key=' > "$tmp/s3fake.port" 2> "$tmp/s3fake.err" & fake=$!
	for i in $(seq 1 100); do [ -s "$tmp/s3fake.port" ] && break; sleep 0.1; done
	export TINHUB_TEST_S3="http://127.0.0.1:$(head -n 1 "$tmp/s3fake.port")" TINHUB_TEST_S3_KEY=tinkey TINHUB_TEST_S3_SECRET='tin/secret+key='
else
	echo "SKIP tinhub S3 pack store (no Go, or the fake S3 does not build)"
fi
for dir in products/tinhub/*/; do
	ls "$dir"*_test.tin >/dev/null 2>&1 || continue
	sh ./tin test "$dir" > "$tmp/test.out" 2>&1 || { cat "$tmp/test.out"; echo "FAIL tinhub tests $dir"; exit 1; }
	echo "PASS tinhub tests $dir"
done
# and against a real S3 server when one is given (CI's MinIO on Linux x86-64): TINHUB_TEST_MINIO is its endpoint, with
# TINHUB_TEST_MINIO_KEY and TINHUB_TEST_MINIO_SECRET
if [ -n "${TINHUB_TEST_MINIO:-}" ]; then
	TINHUB_TEST_S3=$TINHUB_TEST_MINIO TINHUB_TEST_S3_KEY=${TINHUB_TEST_MINIO_KEY:-} TINHUB_TEST_S3_SECRET=${TINHUB_TEST_MINIO_SECRET:-} sh ./tin test products/tinhub/packs/ > "$tmp/test.out" 2>&1 || { cat "$tmp/test.out"; echo "FAIL tinhub pack store on MinIO"; exit 1; }
	grep -q 'TINHUB_TEST_S3 is not set' "$tmp/test.out" && { cat "$tmp/test.out"; echo "FAIL tinhub pack store on MinIO: skipped"; exit 1; }
	echo "PASS tinhub pack store contract on MinIO"
fi
TIN_ROOT=$PWD "$compiler" -o "$tmp/tinhub" products/tinhub/main.tin
"$tmp/tinhub" version > "$tmp/version.out"
grep -q '^tinhub [0-9]' "$tmp/version.out" || { cat "$tmp/version.out"; echo "FAIL tinhub version"; exit 1; }
echo "PASS tinhub version"
if [ -z "${TINHUB_TEST_DB:-}" ]; then
	echo "SKIP tinhub Postgres checks (TINHUB_TEST_DB is not set)"
	exit 0
fi
TIN_ROOT=$PWD "$compiler" -o "$tmp/queue_worker" products/tinhub/tests/programs/queue_worker.tin
sh products/tinhub/tests/queue.sh "$tmp/queue_worker"
TIN_ROOT=$PWD "$compiler" -o "$tmp/repo_driver" products/tinhub/tests/programs/repo_driver.tin
sh products/tinhub/tests/repo.sh "$tmp/repo_driver"
TIN_ROOT=$PWD "$compiler" -o "$tmp/tit" products/tit/main.tin 2>/dev/null
sh products/tinhub/tests/api.sh "$tmp/tinhub" "$tmp/repo_driver" "$tmp/tit"
sh products/tinhub/tests/signin.sh "$tmp/tinhub" "$tmp/repo_driver" "$tmp/tit"
sh products/tinhub/tests/backup.sh "$tmp/tinhub" "$tmp/repo_driver"
sh products/tinhub/tests/phase2.sh "$tmp/tinhub" "$tmp/repo_driver" "$tmp/tit"
# the Tin repo cloned and pushed through tinhub, and short clone timings; TINHUB_BENCH_RUNS, _SECONDS and _CLIENTS
# lengthen them for the numbers in the issues (#1015, #1026)
TIN_ROOT=$PWD "$compiler" -o "$tmp/fetch_load" products/tinhub/tests/programs/fetch_load.tin
TINHUB_BENCH_RUNS=${TINHUB_BENCH_RUNS:-1} TINHUB_BENCH_SECONDS=${TINHUB_BENCH_SECONDS:-5} sh products/tinhub/tests/clones.sh "$tmp/tinhub" "$tmp/repo_driver" "$tmp/tit" "$tmp/fetch_load"
TIN_ROOT=$PWD "$compiler" -o "$tmp/workers_driver" products/tinhub/tests/programs/workers_driver.tin
sh products/tinhub/tests/workers.sh "$tmp/workers_driver"
# the runner: a change's build replayed in the sandbox against examples/checkout.tin's capsules (Linux; skipped without
# user namespaces or Redis)
TIN_ROOT=$PWD "$compiler" -o "$tmp/runner_driver" products/tinhub/tests/programs/runner_driver.tin
TIN_ROOT=$PWD "$compiler" -o "$tmp/runner_probe" products/tinhub/tests/programs/runner_probe.tin
sh products/tinhub/tests/runner.sh "$tmp/runner_driver" "$tmp/runner_probe" "$compiler"
# notifications, webhooks and live updates on two nodes, against bench/ref/tinhub_hooks (Go checks the signatures)
if command -v go > /dev/null 2>&1 && (cd bench/ref/tinhub_hooks && go build -o "$tmp/hooks" main.go) > "$tmp/go.out" 2>&1; then
	TIN_ROOT=$PWD "$compiler" -o "$tmp/notify_node" products/tinhub/tests/programs/notify_node.tin
	sh products/tinhub/tests/notify.sh "$tmp/notify_node" "$tmp/hooks"
else
	echo "SKIP tinhub notify (no Go, or the webhook receiver does not build)"
fi
# the symbol index of this repository takes about a minute: only with TINHUB_TEST_INDEX_TIN=1
if [ "${TINHUB_TEST_INDEX_TIN:-}" = 1 ]; then
	TIN_ROOT=$PWD "$compiler" -o "$tmp/index_tin" products/tinhub/tests/programs/index_tin.tin
	sh products/tinhub/tests/index.sh "$tmp/index_tin"
else
	echo "SKIP tinhub index of the Tin repository (TINHUB_TEST_INDEX_TIN is not 1)"
fi
# a server: not ready before tinhub migrate, ready after, and a request in flight at SIGTERM completes
"$tmp/queue_worker" setup 0 > /dev/null
dbaddr=${TINHUB_TEST_DB%%/*}
export TINHUB_DB_ADDR="$dbaddr" TINHUB_DB_NAME="${TINHUB_TEST_DB#*/}" TINHUB_DB_USER="${TINHUB_TEST_DB_USER:-}" TINHUB_DB_PASSWORD="${TINHUB_TEST_DB_PASSWORD:-}"
export TINHUB_SECRETS_NONCE=test-nonce-secret-0123456789 TINHUB_SECRETS_COOKIE=test-cookie-key-0123456789 TINHUB_TEST_ROUTES=1
export TINHUB_LISTEN=127.0.0.1:18431 TINHUB_SHUTDOWN_DEADLINE=10s TINHUB_PACKS_DIR="$tmp/packs"
"$tmp/queue_worker" reset > /dev/null
if "$tmp/tinhub" run > "$tmp/early.log" 2>&1; then
	echo "FAIL tinhub run started before tinhub migrate"; exit 1
fi
grep -q 'run tinhub migrate' "$tmp/early.log" || { cat "$tmp/early.log"; echo "FAIL tinhub run's refusal"; exit 1; }
# two migrators at once apply each migration once, under the advisory lock
"$tmp/tinhub" migrate > "$tmp/m1.out" &
m1=$!
"$tmp/tinhub" migrate > "$tmp/m2.out"
wait $m1
total=$(cat "$tmp/m1.out" "$tmp/m2.out" | sed -n 's/^tinhub: applied \([0-9]*\) migrations.*/\1/p' | awk '{ s += $1 } END { print s }')
newest=$(ls products/tinhub/db/migrations/*.sql | wc -l | tr -d ' ')
[ "$total" = "$newest" ] || { cat "$tmp/m1.out" "$tmp/m2.out"; echo "FAIL two migrators applied $total migrations, not $newest"; exit 1; }
"$tmp/tinhub" migrate | grep -q 'applied 0 migrations' || { echo "FAIL tinhub migrate twice"; exit 1; }
echo "PASS tinhub migrate (concurrent, idempotent)"
"$tmp/tinhub" packs sweep | grep -q 'swept' || { echo "FAIL tinhub packs sweep"; exit 1; }
"$tmp/tinhub" run > "$tmp/server.log" 2>&1 &
pid=$!
up=0
for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
	if curl -sf "http://$TINHUB_LISTEN/healthz" > /dev/null 2>&1; then up=1; break; fi
	sleep 0.25
done
[ $up = 1 ] || { cat "$tmp/server.log"; echo "FAIL tinhub run did not start"; exit 1; }
curl -sf "http://$TINHUB_LISTEN/readyz" | grep -q '^ready' || { echo "FAIL tinhub readyz"; exit 1; }
curl -s "http://$TINHUB_LISTEN/test/wait?ms=2000" > "$tmp/slow.out" &
slow=$!
sleep 0.5
kill -TERM "$pid"
wait "$slow"
grep -q 'waited 2000ms' "$tmp/slow.out" || { cat "$tmp/slow.out" "$tmp/server.log"; echo "FAIL a request in flight at SIGTERM did not complete"; exit 1; }
wait "$pid" || { cat "$tmp/server.log"; echo "FAIL tinhub run did not exit cleanly after SIGTERM"; exit 1; }
pid=
echo "PASS tinhub server (readiness, drain)"
