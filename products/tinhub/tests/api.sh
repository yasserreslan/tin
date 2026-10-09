#!/bin/sh
# The JSON API's contract tests (#1019) and the protocol's (#1018) against TINHUB_TEST_DB. Usage: api.sh TINHUB
# REPO_DRIVER TIT. A server runs on the repository repo_driver setup makes (ada/tin, public, main at c1); every
# endpoint is checked for its answer's shape, a private repository is a 404 everywhere, the rate limit answers 429
# with Retry-After, and tit clones from the server.
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
	echo "FAIL tinhub api: $*"
	[ -f "$tmp/hub.log" ] && tail -n 20 "$tmp/hub.log"
	exit 1
}

"$driver" setup "$tmp/node" > /dev/null
dbaddr=${TINHUB_TEST_DB%%/*}
export TINHUB_DB_ADDR="$dbaddr" TINHUB_DB_NAME="${TINHUB_TEST_DB#*/}" TINHUB_DB_USER="${TINHUB_TEST_DB_USER:-}" TINHUB_DB_PASSWORD="${TINHUB_TEST_DB_PASSWORD:-}"
export TINHUB_SECRETS_NONCE=test-nonce-secret-0123456789 TINHUB_SECRETS_COOKIE=test-cookie-key-0123456789
export TINHUB_LISTEN=127.0.0.1:18433 TINHUB_PACKS_DIR="$tmp/node" TINHUB_PACKS_SWEEP=0
base=http://$TINHUB_LISTEN
start() {
	"$hub" run > "$tmp/hub.log" 2>&1 &
	pid=$!
	for i in $(seq 1 40); do
		curl -sf "$base/healthz" > /dev/null 2>&1 && return 0
		sleep 0.25
	done
	fail "the server did not start"
}
stop() {
	kill -TERM "$pid" 2>/dev/null || true
	wait "$pid" 2>/dev/null || true
	pid=
}
start

# get PATH JQ: the answer to GET PATH is 200 JSON and JQ holds on it
get() {
	code=$(curl -s -o "$tmp/out.json" -w '%{http_code}' "$base$1")
	[ "$code" = 200 ] || fail "GET $1: $code $(cat "$tmp/out.json")"
	jq -e "$2" "$tmp/out.json" > /dev/null || fail "GET $1: $(cat "$tmp/out.json") does not hold $2"
}
# status PATH CODE ERROR: the answer is CODE with {"code": ERROR, "message": ...}
status() {
	code=$(curl -s -o "$tmp/out.json" -w '%{http_code}' "$base$1")
	[ "$code" = "$2" ] || fail "GET $1: $code, not $2"
	jq -e ".code == \"$3\" and (.message | length > 0)" "$tmp/out.json" > /dev/null || fail "GET $1: $(cat "$tmp/out.json") is not a $3 error"
}

r=/api/v1/repos/ada/tin
get "$r" '.owner == "ada" and .name == "tin" and .visibility == "public" and .default_branch == "main" and .size_bytes > 0'
get /api/v1/owners/ada/repos '(.repos | length) == 1 and .repos[0].name == "tin" and .next == ""'
get "$r/refs" '(.refs | length) == 1 and .refs[0].name == "refs/heads/main" and (.refs[0].target | length) == 64'
main=$(jq -r '.refs[0].target' "$tmp/out.json")
get "$r/refs?prefix=refs/tags/" '(.refs | length) == 0'
get "$r/changes" '(.changes | length) == 1 and .changes[0].newest.version == 1 and .changes[0].newest.commit == "'"$main"'" and .next == ""'
change=$(jq -r '.changes[0].change' "$tmp/out.json")
get "$r/changes?limit=1" '(.changes | length) == 1'
get "$r/changes/$change" '.change == "'"$change"'" and (.versions | length) == 1 and .versions[0].op == 1'
get "$r/changes/$change/diff" '.from == 0 and .to == 1 and (.files | length) == 1 and .files[0].kind == "added" and .files[0].path == "a.txt"'
blob=$(jq -r '.files[0].new_blob' "$tmp/out.json")
get "$r/stacks/ada" '(.changes | length) == 0'
get "$r/commits/$main" '.id == "'"$main"'" and (.tree | length) == 64 and (.parents | length) == 0 and .change == "'"$change"'" and .author.email == "ada@example.com"'
treeid=$(jq -r '.tree' "$tmp/out.json")
get "$r/trees/$treeid" '(.entries | length) == 1 and .entries[0].name == "a.txt" and .entries[0].mode == "file" and .entries[0].id == "'"$blob"'"'
body=$(curl -sf "$base$r/blobs/$blob") || fail "GET blob"
[ "$body" = one ] || fail "the blob is '$body', not 'one'"
status /api/v1/repos/ada/nope 404 not_found
status "$r/commits/$treeid" 404 not_found
status "$r/blobs/0000000000000000000000000000000000000000000000000000000000000000" 404 not_found
status "$r/changes?cursor=nonsense" 400 bad_request
echo "PASS tinhub api: every endpoint's answer"

# tit clones from the server, unmodified
(cd "$tmp" && "$tit" clone "$base/ada/tin" clone > clone.out 2>&1) || { cat "$tmp/clone.out"; fail "tit clone"; }
[ "$(cat "$tmp/clone/a.txt")" = one ] || fail "the clone's a.txt"
echo "PASS tinhub protocol: tit clone"

# a private repository is a 404 to anyone outside it, on the protocol and the API alike
"$driver" private > /dev/null
status "$r" 404 not_found
status "$r/refs" 404 not_found
get /api/v1/owners/ada/repos '(.repos | length) == 0'
code=$(curl -s -o /dev/null -w '%{http_code}' "$base/ada/tin/tit/v1/heads")
[ "$code" = 404 ] || fail "heads of a private repository: $code, not 404"
echo "PASS tinhub api: a private repository is not found"
stop

# the rate limit (Redis): a third request in the window is refused
if start_redis; then
	export TINHUB_REDIS_ADDR=127.0.0.1:$port TINHUB_API_RATE=2
	start
	curl -s -o /dev/null "$base/api/v1/owners/ada/repos"
	curl -s -o /dev/null "$base/api/v1/owners/ada/repos"
	code=$(curl -s -D "$tmp/headers" -o "$tmp/out.json" -w '%{http_code}' "$base/api/v1/owners/ada/repos")
	[ "$code" = 429 ] || fail "the third request in the window: $code, not 429"
	grep -qi '^retry-after: [0-9]' "$tmp/headers" || fail "429 without Retry-After"
	jq -e '.code == "rate_limited"' "$tmp/out.json" > /dev/null || fail "429's body"
	stop
	stop_redis
	echo "PASS tinhub api: the rate limit"
else
	echo "SKIP tinhub api rate limit (no redis-server or docker)"
fi
