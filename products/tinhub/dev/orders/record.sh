#!/bin/sh
# Records orders' failures as capsules (design/tinhub.md §12.1): the payment service in main.tin against a real
# Postgres, Redis and the rates website api.frankfurter.dev (so it needs the network). Usage: record.sh COMPILER SPOOL,
# with POSTGRES_ADDR, POSTGRES_USER, POSTGRES_PASSWORD and POSTGRES_DATABASE (a database this script empties: schema.sql
# drops and makes its two tables), REDIS_ADDR, TIN_REPLAY_RECIPIENTS and TIN_REPLAY_SIGNING_KEY set; ORDERS_PORT
# (default 9198) is the port the service listens on. Needs psql and redis-cli.
#
# Orders 101 and 102 are paid and record nothing. 201 and 202 are in ARS, which the website does not price: a panic
# (two capsules, one group). 301 and 302 are each paid by two requests at once: the second insert breaks payments'
# primary key, a 500 (two capsules, another group). 401 and 402 are free: a panic after the payment is written (two
# capsules, a third group).
set -eu
compiler=$1
spool=$2
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../../.." && pwd)
port=${ORDERS_PORT:-9198}
tmp=$(mktemp -d)
svc=
cleanup() {
	[ -n "$svc" ] && kill "$svc" 2>/dev/null || true
	rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
PGPASSWORD=$POSTGRES_PASSWORD psql -q -h "${POSTGRES_ADDR%:*}" -p "${POSTGRES_ADDR##*:}" -U "$POSTGRES_USER" "$POSTGRES_DATABASE" -v ON_ERROR_STOP=1 -f "$here/schema.sql" > /dev/null 2>&1
redis-cli -h "${REDIS_ADDR%:*}" -p "${REDIS_ADDR##*:}" del fx:EUR fx:GBP fx:ARS > /dev/null
TIN_ROOT=$root "$compiler" -o "$tmp/orders" "$here/main.tin"
mkdir -p "$spool"
PORT=$port TIN_REPLAY_DIR=$spool TIN_REPLAY_KEY=${TIN_REPLAY_KEY:-$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')} \
	TIN_REPLAY_SUMMARY_PANIC=1 "$tmp/orders" > "$tmp/orders.log" 2>&1 &
svc=$!
pay() {
	curl -s -o /dev/null -m 15 -X POST "http://127.0.0.1:$port/orders/$1/pay"
}
for i in $(seq 1 40); do
	curl -s -o /dev/null -X POST "http://127.0.0.1:$port/orders/0/pay" 2>/dev/null && break
	sleep 0.25
done
pay 101
pay 102
pay 201
pay 202
# two clicks at once: both read the order before either inserts (the website call between gives the race its window)
for o in 301 302; do
	pay $o &
	first=$!
	pay $o
	wait "$first"
done
pay 401
pay 402
sleep 0.5
kill "$svc"
wait "$svc" 2>/dev/null || true
svc=
n=$(ls "$spool" | grep -c '\.tcap$' || true)
[ "$n" = 6 ] || { cat "$tmp/orders.log" >&2; echo "record: $n capsules, not 6" >&2; exit 1; }
echo "recorded $n capsules in $spool"
