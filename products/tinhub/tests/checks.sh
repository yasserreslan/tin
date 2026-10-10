#!/bin/sh
# Replay checks end to end (design/tinhub.md §12.1) against TINHUB_TEST_DB, on Linux. Usage: checks.sh RUNNER_DRIVER
# COMPILER. dev/shop records its two failures (a panic on a declined cart, a 500 while the payment service is down) as
# capsules sealed for the runner's key; the repository holds shop on main and five fix branches; each branch is
# checked against both failure groups in the runner's sandbox, and each capsule's verdict is the one the branch earns:
# the fix passes, a retry or a new lookup diverges, refusing early leaves a recorded call not made, and a wrong fix still
# panics. Skipped where the sandbox cannot run (not Linux, no user namespaces) or there is no Redis.
set -eu
driver=$1
compiler=$2
root=$PWD
tmp=$(mktemp -d)
. "$(dirname "$0")/lib.sh"
cleanup() {
	stop_redis
	rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
fail() {
	echo "FAIL tinhub checks: $*"
	[ -f "$tmp/checks.out" ] && cat "$tmp/checks.out"
	exit 1
}
if [ "$(uname -s)" != Linux ]; then
	echo "SKIP tinhub checks (the sandbox is Linux only)"
	exit 0
fi
if ! why=$("$driver" sandbox "$root" "$tmp/sandbox" 2>&1); then
	echo "SKIP tinhub checks (the sandbox cannot run here: $why)"
	exit 0
fi
if ! start_redis > /dev/null 2>&1; then
	echo "SKIP tinhub checks (no redis-server or docker for the recording)"
	exit 0
fi
shop=products/tinhub/dev/shop
mkdir -p "$tmp/secrets"
sh ./tin replay key "$tmp/secrets/runner.key" > "$tmp/runner.pub"
REDIS_ADDR=127.0.0.1:$port TIN_REPLAY_RECIPIENTS=$(cat "$tmp/runner.pub") \
	TIN_REPLAY_SIGNING_KEY=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n') \
	SHOP_PORT=19196 PAYMENTS_PORT=19197 sh $shop/record.sh "$compiler" "$tmp/spool" > /dev/null || fail "recording shop"
"$driver" shop "$tmp/packs" $shop > /dev/null
"$driver" upload "$tmp/packs" "$tmp/spool" > /dev/null
# two failures, in two groups, or one per request path where capsules carry no route pattern; before the owner opts in,
# a check queues nothing
"$driver" check "$tmp/packs" main > "$tmp/first"
groups=$(wc -l < "$tmp/first")
[ "$groups" -ge 2 ] || fail "$groups failure groups, not 2 or more"
[ "$(sort -u "$tmp/first")" = 0 ] || fail "a check was queued before the opt-in: $(cat "$tmp/first")"
"$driver" optin "$tmp/secrets/runner.key" PAYMENTS_URL=http://127.0.0.1:19197 > /dev/null
for b in main fix/declined-cart fix/retry-charge fix/coupon-lookup fix/limit-cart-size fix/split-on-colon; do
	"$driver" check "$tmp/packs" "$b" > /dev/null
done
"$driver" checks "$tmp/packs" "$root" "$tmp/secrets/runner.key" > "$tmp/checks.out" 2>&1 || fail "the checks failed"

# verdicts BRANCH KIND: the verdict lines of BRANCH's checks of the panic's groups or the 500's (KIND panic or status)
verdicts() {
	awk -v want="check $1 $2 " 'index($0, "check ") == 1 { on = index($0, want) == 1; next } on && /^verdict / { print }' "$tmp/checks.out" | sort | uniq -c | sed 's/^ *//'
}
expect() {
	got=$(verdicts "$1" "$2")
	[ "$got" = "$3" ] || fail "$1 against the $2 group: got
$got
want
$3"
}
expect main panic "3 verdict panicked panicked in checkout: the recorded panic"
expect main status "2 verdict failing still fails: status 500"
expect fix/declined-cart panic "3 verdict passed passed: 402, recorded 500 after a panic"
expect fix/declined-cart status "2 verdict failing still fails: status 503"
expect fix/retry-charge panic "3 verdict diverged diverged at effect 3: an extra wire.http@1"
expect fix/retry-charge status "2 verdict diverged diverged at effect 3: an extra wire.http@1"
expect fix/coupon-lookup panic "3 verdict diverged diverged at effect 2: redis@1 where production made wire.http@1"
expect fix/limit-cart-size panic "3 verdict diverged diverged: answered without 1 recorded call"
expect fix/limit-cart-size status "2 verdict failing still fails: status 500"
expect fix/split-on-colon panic "3 verdict panicked panicked in checkout: the recorded panic"
grep -q '^why Fixed. Production answered 500 after a panic; this build answers 402' "$tmp/checks.out" || fail "the fix's verdict is not explained"
echo "PASS tinhub checks: shop's fix branches against its two failure groups (passed, diverged three ways, still panicking)"
if grep -q 'apples\|declined: card' "$tmp/checks.out"; then fail "a recorded request's data is in a verdict"; fi
[ -z "$(ls -A "$tmp/packs/runner")" ] || fail "the checks left files in $tmp/packs/runner"
echo "PASS tinhub checks: no request data in the verdicts, and no files left"
