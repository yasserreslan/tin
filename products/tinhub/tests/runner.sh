#!/bin/sh
# The runner's process test (#1028) against TINHUB_TEST_DB, on Linux. Usage: runner.sh RUNNER_DRIVER RUNNER_PROBE COMPILER
# examples/checkout.tin, serving carts from a Redis of its own, records the capsule of every request, sealed for the
# runner's key; a change that adds a SQL query to it is built and replayed in the runner's sandboxes against its base
# (examples/checkout.tin itself), and shows as one "different calls at effect N" group (the requests that meet the
# query) beside the requests that are the same. The replays' sandbox reaches itself on its own loopback and nothing
# else, and the runner's private key is only ever written inside a run's sandbox directory, then removed.
# Skipped where the sandbox cannot run (not Linux, no user namespaces) or there is no Redis.
set -eu
driver=$1
probe=$2
compiler=$3
root=$PWD
tmp=$(mktemp -d)
app=
watcher=
. "$(dirname "$0")/lib.sh"
cleanup() {
	[ -n "$app" ] && kill "$app" 2>/dev/null || true
	[ -n "$watcher" ] && kill "$watcher" 2>/dev/null || true
	stop_redis
	rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
fail() {
	echo "FAIL tinhub runner: $*"
	[ -f "$tmp/run.out" ] && tail -n 30 "$tmp/run.out"
	exit 1
}

if [ "$(uname -s)" != Linux ]; then
	echo "SKIP tinhub runner (the sandbox is Linux only)"
	exit 0
fi
if ! why=$("$driver" sandbox "$root" "$tmp/sandbox" 2>&1); then
	echo "SKIP tinhub runner (the sandbox cannot run here: $why)"
	exit 0
fi
if ! start_redis > /dev/null 2>&1; then
	echo "SKIP tinhub runner (no redis-server or docker for the recording)"
	exit 0
fi
rcli() {
	if [ -n "$container" ]; then docker exec "$container" redis-cli "$@"; else redis-cli -p $port "$@"; fi
}

# the runner's key, as runner.key = file:… holds it; only its public key goes to the recording server
mkdir -p "$tmp/secrets"
sh ./tin replay key "$tmp/secrets/runner.key" > "$tmp/runner.pub"
signing=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')
hmac=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')

# record: carts 1 to 6 exist (the charge fails: 500), 7 and 8 do not (404); every request is sampled
TIN_ROOT=$root "$compiler" -o "$tmp/checkout" examples/checkout.tin
for i in 1 2 3 4 5 6; do rcli set "cart:$i" "apples-$i" > /dev/null; done
mkdir -p "$tmp/spool"
REDIS_ADDR=127.0.0.1:$port PAYMENTS_URL=http://127.0.0.1:1 TIN_REPLAY_DIR="$tmp/spool" TIN_REPLAY_KEY=$hmac \
	TIN_REPLAY_RECIPIENTS=$(cat "$tmp/runner.pub") TIN_REPLAY_SIGNING_KEY=$signing TIN_REPLAY_SAMPLE=1 \
	"$tmp/checkout" > "$tmp/checkout.log" 2>&1 &
app=$!
up=0
for i in $(seq 1 40); do
	if curl -s -o /dev/null -X POST http://127.0.0.1:9186/checkout/0 2>/dev/null; then up=1; break; fi
	sleep 0.25
done
[ $up = 1 ] || { cat "$tmp/checkout.log"; fail "examples/checkout.tin did not start"; }
for i in 1 2 3 4 5 6 7 8; do curl -s -o /dev/null -X POST "http://127.0.0.1:9186/checkout/$i"; done
sleep 0.5
kill "$app"
wait "$app" 2>/dev/null || true
app=
recorded=$(ls "$tmp/spool" | grep -c '\.tcap$' || true)
[ "$recorded" = 9 ] || fail "examples/checkout.tin recorded $recorded capsules, not 9"

# the change: examples/checkout.tin with an INSERT before the charge
awk '
/^import "anvil"$/ { print; print "import \"postgres\""; next }
/^let cache = redis.Open/ { print; print "let orders = postgres.Open(postgres.Options{Addr: quarry.Getenv(\"ORDERS_ADDR\")})"; next }
/let r = wire.Post\(/ {
	print "\t_ = orders.Exec(\"INSERT INTO orders (cart) VALUES ({id})\") catch err {"
	print "\t\tw.Status(500)"
	print "\t\tw.Text(\"order: \" + fault.Message(err) + \"\\n\")"
	print "\t\treturn"
	print "\t}"
	print
	next
}
{ print }' examples/checkout.tin > "$tmp/checkout_sql.tin"
added=$(diff examples/checkout.tin "$tmp/checkout_sql.tin" | grep -c '^>' || true)
[ "$added" = 7 ] || fail "the change to examples/checkout.tin adds $added lines, not 7 (did the example change?)"

"$driver" setup "$tmp/packs" examples/checkout.tin "$tmp/checkout_sql.tin" > /dev/null
"$driver" upload "$tmp/packs" "$tmp/spool" > /dev/null
"$driver" optin "$tmp/secrets/runner.key" PAYMENTS_URL=http://127.0.0.1:1 REDIS_ADDR=127.0.0.1:$port > /dev/null
run=$("$driver" request)
[ "$run" -gt 0 ] || fail "no run was queued"

# while the run goes, every file under the test's directory (but the secret's own) that holds the key is noted
key=$(sed 's/^tinreplaykey1://' "$tmp/secrets/runner.key")
mark=$(printf '%s' "$key" | cut -c1-48)
(
	while :; do
		grep -rlF "$mark" "$tmp" --exclude-dir=secrets --exclude=seen 2>/dev/null >> "$tmp/seen" || true
		sleep 0.05
	done
) &
watcher=$!
"$driver" run "$tmp/packs" "$root" "$tmp/secrets/runner.key" > "$tmp/run.out" 2>&1 || fail "the run failed"
kill "$watcher" 2>/dev/null || true
wait "$watcher" 2>/dev/null || true
watcher=

grep -q '^run done capsules 9 ' "$tmp/run.out" || fail "the run did not replay 9 capsules"
calls=$(grep -c '^group calls ' "$tmp/run.out" || true)
[ "$calls" = 1 ] || fail "$calls groups of different calls, not 1"
grep -Eq '^group calls 6 different calls at effect [0-9]+: got postgres@1, recorded wire\.http@1$' "$tmp/run.out" ||
	fail "the six requests that meet the query are not one \"different calls at effect N\" group"
grep -q '^group same 3 same$' "$tmp/run.out" || fail "the three requests that do not meet the query are not the same"
[ "$(grep -c '^group ' "$tmp/run.out")" = 2 ] || fail "other groups than the query's and the same"
label=$(grep '^group calls ' "$tmp/run.out" | sed 's/^group calls 6 //')
"$driver" panel > "$tmp/panel.out"
[ "$(head -n 1 "$tmp/panel.out")" = 200 ] || fail "the behaviour panel answered $(head -n 1 "$tmp/panel.out")"
tail -n 1 "$tmp/panel.out" | jq -e --arg l "$label" '.run.state == "done" and (.groups | length) == 2 and .groups[0].outcome == "calls" and .groups[0].count == 6 and .groups[0].label == $l and .groups[0].effect >= 0 and .groups[1].outcome == "same"' > /dev/null ||
	fail "the behaviour panel: $(tail -n 1 "$tmp/panel.out")"
echo "PASS tinhub runner: a change adding a SQL query to examples/checkout.tin is one group ($label)"

# the key: seen only as the sandbox's scratch file while the replays ran, and nowhere after
[ -s "$tmp/seen" ] || fail "the key was never seen in a sandbox directory (the watcher saw nothing)"
outside=$(sort -u "$tmp/seen" | grep -v "^$tmp/packs/runner/$run-[0-9a-f]*/scratch/runner.key\$" || true)
[ -z "$outside" ] || fail "the key was written outside the runner's sandbox: $outside"
left=$(grep -rlF "$mark" "$tmp" --exclude-dir=secrets --exclude=seen 2>/dev/null || true)
[ -z "$left" ] || fail "the key is still in $left"
[ -z "$(ls -A "$tmp/packs/runner")" ] || fail "the run left files in $tmp/packs/runner"
if command -v pg_dump > /dev/null 2>&1; then
	dbhost=${TINHUB_TEST_DB%%:*}
	dbport=${TINHUB_TEST_DB#*:}
	dbport=${dbport%%/*}
	PGPASSWORD="${TINHUB_TEST_DB_PASSWORD:-}" pg_dump -h "$dbhost" -p "$dbport" -U "${TINHUB_TEST_DB_USER:-}" "${TINHUB_TEST_DB#*/}" > "$tmp/dump.sql" ||
		fail "pg_dump"
	if grep -qF "$mark" "$tmp/dump.sql"; then fail "the key is in Postgres"; fi
	if grep -qF "apples-" "$tmp/dump.sql"; then fail "a recorded request's data is in Postgres"; fi
fi
if grep -qF "$mark" "$tmp/run.out"; then fail "the key is in the runner's log"; fi
echo "PASS tinhub runner: the key was only in the run's sandbox directory, and is gone"

# no network: in a replay's sandbox the probe reaches itself, but neither the host's Redis nor the internet
"$driver" probe "$tmp/sandbox" "$root" "$probe" "127.0.0.1:$port" > "$tmp/probe.out" 2>&1 || true
grep -q '^self ok$' "$tmp/probe.out" || fail "the replay's sandbox has no loopback: $(cat "$tmp/probe.out")"
grep -q "^dial 127.0.0.1:$port fail" "$tmp/probe.out" || fail "the replay's sandbox reached the host: $(cat "$tmp/probe.out")"
"$driver" probe "$tmp/sandbox" "$root" "$probe" 1.1.1.1:53 > "$tmp/probe2.out" 2>&1 || true
grep -q '^dial 1.1.1.1:53 fail' "$tmp/probe2.out" || fail "the replay's sandbox reached the internet: $(cat "$tmp/probe2.out")"
echo "PASS tinhub runner: the sandbox reaches its own loopback and no network"
