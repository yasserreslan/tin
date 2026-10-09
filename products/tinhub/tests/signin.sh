#!/bin/sh
# Accounts end to end (#1012, #1018) against TINHUB_TEST_DB. Usage: signin.sh TINHUB REPO_DRIVER TIT
#   - tinhub admin invite makes a code; tit key add registers bob's key with it, once;
#   - bob, who can read the public ada/tin, is refused a push; granted write, his push lands and the API shows it;
#   - a browser asks for a login request, bob approves it with tit login, the browser claims it and gets a session
#     that reads the repository once it is private (anonymous callers get 404); signing out ends it.
set -eu
hub=$1
driver=$2
tit=$3
tmp=$(mktemp -d)
pid=
. "$(dirname "$0")/lib.sh"
cleanup() {
	[ -n "$pid" ] && kill "$pid" 2>/dev/null || true
	stop_redis
	rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
fail() {
	echo "FAIL tinhub sign-in: $*"
	[ -f "$tmp/hub.log" ] && tail -n 20 "$tmp/hub.log"
	exit 1
}
start_redis || fail "sessions need Redis (redis-server, or docker)"
"$driver" setup "$tmp/node" > /dev/null
dbaddr=${TINHUB_TEST_DB%%/*}
export TINHUB_DB_ADDR="$dbaddr" TINHUB_DB_NAME="${TINHUB_TEST_DB#*/}" TINHUB_DB_USER="${TINHUB_TEST_DB_USER:-}" TINHUB_DB_PASSWORD="${TINHUB_TEST_DB_PASSWORD:-}"
export TINHUB_SECRETS_NONCE=test-nonce-secret-0123456789 TINHUB_SECRETS_COOKIE=test-cookie-key-0123456789
export TINHUB_LISTEN=127.0.0.1:18434 TINHUB_PACKS_DIR="$tmp/node" TINHUB_PACKS_SWEEP=0 TINHUB_REDIS_ADDR="127.0.0.1:$port"
base=http://$TINHUB_LISTEN
export TINHUB_PUBLIC_URL="$base"
"$hub" run > "$tmp/hub.log" 2>&1 &
pid=$!
up=0
for i in $(seq 1 40); do
	curl -sf "$base/healthz" > /dev/null 2>&1 && { up=1; break; }
	sleep 0.25
done
[ $up = 1 ] || fail "the server did not start"

export TIT_NO_PAGER=1
bob() {
	HOME="$tmp/bob" XDG_CONFIG_HOME="$tmp/bob" "$tit" "$@"
}
mkdir -p "$tmp/bob"
bob config set --user user.name Bob
bob config set --user user.email bob@example.com
bob key > /dev/null 2>&1

# an invite lets bob's key in, once
"$hub" admin invite bob@example.com > "$tmp/invite.out" || fail "tinhub admin invite: $(cat "$tmp/invite.out")"
code=$(sed -n 's/^invite for bob@example.com: \(.*\)$/\1/p' "$tmp/invite.out")
[ -n "$code" ] || fail "the invite: $(cat "$tmp/invite.out")"
bob key add "$base" "$code" > "$tmp/out" 2>&1 || fail "tit key add: $(cat "$tmp/out")"
grep -q "^Added the key of bob@example.com" "$tmp/out" || fail "tit key add said: $(cat "$tmp/out")"
if bob key add "$base" "$code" > "$tmp/out" 2>&1; then
	fail "an invite was used twice"
fi
echo "PASS tinhub sign-in: an invite adds a key, once"

# bob reads the public repository but cannot push to it until he is granted write
curl -sf "$base/api/v1/repos/ada/tin/refs" | jq -r ".refs[0].target" > "$tmp/main.before"
(cd "$tmp" && bob clone "$base/ada/tin" work > "$tmp/out" 2>&1) || fail "bob's clone: $(cat "$tmp/out")"
cd "$tmp/work"
printf 'two\n' > b.txt
bob add b.txt
bob commit -m second > /dev/null
if bob push > "$tmp/out" 2>&1; then
	fail "a reader pushed"
fi
grep -q "not push" "$tmp/out" || fail "the refused push said: $(cat "$tmp/out")"
"$driver" grant bob@example.com write
bob push > "$tmp/out" 2>&1 || fail "bob's push: $(cat "$tmp/out")"
cd "$tmp"
refs=$(curl -sf "$base/api/v1/repos/ada/tin/refs") || fail "GET refs after the push"
echo "$refs" | jq -e '.refs[0].name == "refs/heads/main"' > /dev/null || fail "refs: $refs"
before=$(cat "$tmp/main.before")
echo "$refs" | jq -e --arg b "$before" '.refs[0].target != $b' > /dev/null || fail "main did not move: $refs"
echo "PASS tinhub protocol: a reader's push is refused, a writer's lands"

# a browser signs in with bob's key, and its session reads the private repository
"$driver" private
code=$(curl -s -o /dev/null -w '%{http_code}' "$base/api/v1/repos/ada/tin")
[ "$code" = 404 ] || fail "anonymous read of a private repository: $code"
curl -s -c "$tmp/jar" -X POST "$base/tit/v1/login" > "$tmp/req.json"
login=$(jq -r .code "$tmp/req.json")
[ ${#login} -eq 9 ] || fail "the login request: $(cat "$tmp/req.json")"
claim=$(curl -s -b "$tmp/jar" -o /dev/null -w '%{http_code}' -X POST "$base/tit/v1/login/$login/session")
[ "$claim" != 200 ] || fail "a pending login request was claimed"
(cd "$tmp/work" && bob login --code "$login") > "$tmp/out" 2>&1 || fail "tit login: $(cat "$tmp/out")"
curl -s "$base/tit/v1/login/$login" | jq -e '.state == "approved" and .email == "bob@example.com"' > /dev/null || fail "the login request is not approved"
claim=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$base/tit/v1/login/$login/session")
[ "$claim" != 200 ] || fail "a login request was claimed without its claim cookie"
curl -sf -b "$tmp/jar" -c "$tmp/jar" -X POST "$base/tit/v1/login/$login/session" > "$tmp/out" || fail "the claim: $(cat "$tmp/out")"
code=$(curl -s -b "$tmp/jar" -o "$tmp/out" -w '%{http_code}' "$base/api/v1/repos/ada/tin")
[ "$code" = 200 ] || fail "a signed-in read of the private repository: $code $(cat "$tmp/out")"
curl -sf -b "$tmp/jar" -c "$tmp/jar" -X POST "$base/tit/v1/logout" > /dev/null || fail "sign-out"
code=$(curl -s -b "$tmp/jar" -o /dev/null -w '%{http_code}' "$base/api/v1/repos/ada/tin")
[ "$code" = 404 ] || fail "a read after sign-out: $code"
echo "PASS tinhub sign-in: a browser session from tit login, and sign-out"
