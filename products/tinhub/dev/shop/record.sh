#!/bin/sh
# Records shop's failures as capsules (design/tinhub.md §12.1): the checkout service in main.tin, its payment service
# (payments.tin) and the carts (carts.tin) in the Redis at REDIS_ADDR. Usage: record.sh COMPILER SPOOL, with
# TIN_REPLAY_RECIPIENTS and TIN_REPLAY_SIGNING_KEY set (the readers the capsules are sealed for, and the server's signing
# key); SHOP_PORT and PAYMENTS_PORT (default 9196, 9197) are the ports the two services listen on.
#
# Cart 1 is paid and recorded nothing. Carts 2 to 4 hold more than three items: the payment service declines them (402)
# and shop panics reading a receipt the decline does not have (three capsules, one failure group). Carts 5 and 6 are
# charged while the payment service is down: shop answers 500 (two capsules, another group).
set -eu
compiler=$1
spool=$2
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../../.." && pwd)
shop_port=${SHOP_PORT:-9196}
pay_port=${PAYMENTS_PORT:-9197}
tmp=$(mktemp -d)
shop=
pay=
cleanup() {
	[ -n "$shop" ] && kill "$shop" 2>/dev/null || true
	[ -n "$pay" ] && kill "$pay" 2>/dev/null || true
	rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
for p in main payments carts; do TIN_ROOT=$root "$compiler" -o "$tmp/$p" "$here/$p.tin"; done
"$tmp/carts" > /dev/null
up() {
	for i in $(seq 1 40); do
		curl -s -o /dev/null -X POST "$1" 2>/dev/null && return 0
		sleep 0.25
	done
	return 1
}
PORT=$pay_port "$tmp/payments" > "$tmp/payments.log" 2>&1 &
pay=$!
up "http://127.0.0.1:$pay_port/charge" || { cat "$tmp/payments.log" >&2; echo "record: the payment service did not start" >&2; exit 1; }
mkdir -p "$spool"
PORT=$shop_port PAYMENTS_URL=http://127.0.0.1:$pay_port TIN_REPLAY_DIR=$spool TIN_REPLAY_KEY=${TIN_REPLAY_KEY:-$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')} \
	TIN_REPLAY_SUMMARY_PANIC=1 "$tmp/main" > "$tmp/shop.log" 2>&1 &
shop=$!
up "http://127.0.0.1:$shop_port/checkout/0" || { cat "$tmp/shop.log" >&2; echo "record: shop did not start" >&2; exit 1; }
for i in 1 2 3 4; do curl -s -o /dev/null -X POST "http://127.0.0.1:$shop_port/checkout/$i"; done
kill "$pay"
wait "$pay" 2>/dev/null || true
pay=
for i in 5 6; do curl -s -o /dev/null -X POST "http://127.0.0.1:$shop_port/checkout/$i"; done
sleep 0.5
kill "$shop"
wait "$shop" 2>/dev/null || true
shop=
n=$(ls "$spool" | grep -c '\.tcap$' || true)
[ "$n" = 5 ] || { cat "$tmp/shop.log" >&2; echo "record: $n capsules, not 5" >&2; exit 1; }
echo "recorded $n capsules in $spool"
