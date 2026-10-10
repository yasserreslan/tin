#!/bin/sh
# The web UI's server side (#1081, #1093) against TINHUB_TEST_DB. Usage: web.sh TINHUB REPO_DRIVER. Page URLs answer the
# app shell with its security headers, the embedded assets are served by build with validators and gzip, unknown API
# and asset paths stay 404, and the UI's JavaScript unit tests run when Node is installed.
set -eu
hub=$1
driver=$2
tmp=$(mktemp -d)
pid=
root=$(cd "$(dirname "$0")/../../.." && pwd)
cleanup() {
	[ -n "$pid" ] && kill "$pid" 2>/dev/null || true
	rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
fail() {
	echo "FAIL tinhub web: $*"
	[ -f "$tmp/hub.log" ] && tail -n 20 "$tmp/hub.log"
	exit 1
}

"$driver" setup "$tmp/node" > /dev/null
export TINHUB_DB_ADDR="${TINHUB_TEST_DB%%/*}" TINHUB_DB_NAME="${TINHUB_TEST_DB#*/}" TINHUB_DB_USER="${TINHUB_TEST_DB_USER:-}" TINHUB_DB_PASSWORD="${TINHUB_TEST_DB_PASSWORD:-}"
export TINHUB_SECRETS_NONCE=test-nonce-secret-0123456789 TINHUB_SECRETS_COOKIE=test-cookie-key-0123456789
export TINHUB_LISTEN=127.0.0.1:18437 TINHUB_PACKS_STORE=dir TINHUB_PACKS_DIR="$tmp/node" TINHUB_PACKS_SWEEP=0 TINHUB_PUBLIC_URL=https://hub.example.com
unset TINHUB_WEB_DIR
base=http://$TINHUB_LISTEN
"$hub" run > "$tmp/hub.log" 2>&1 &
pid=$!
for i in $(seq 1 40); do
	curl -sf "$base/healthz" > /dev/null 2>&1 && break
	[ "$i" = 40 ] && fail "the server did not start"
	sleep 0.25
done

# head PATH: the status line and headers of GET PATH in $tmp/h, the body in $tmp/b
head_() {
	curl -s -D "$tmp/h" -o "$tmp/b" "$@"
}
has() {
	grep -qi "$1" "$tmp/h" || fail "$2: no '$1' in $(cat "$tmp/h")"
}

# a page URL is the shell, with the build and the site filled in and its security headers
for page in / /explore /ada /ada/tin /ada/tin/change/abc/files /ada/tin/settings/webhooks; do
	head_ "$base$page"
	has '^HTTP/1.1 200' "$page"
	has '^content-type: text/html' "$page"
	has "^content-security-policy: default-src 'self'; script-src 'self'; style-src 'self'" "$page"
	has '^x-frame-options: DENY' "$page"
	has '^x-content-type-options: nosniff' "$page"
	grep -q '%BUILD%\|%PUBLIC_URL%' "$tmp/b" && fail "$page: placeholders left in the shell"
	grep -q 'https://hub.example.com' "$tmp/b" || fail "$page: the public URL is not in the shell"
done
build=$(sed -n 's/.*\/-\/\([0-9a-f]\{12\}\)\/js\/main\.js.*/\1/p' "$tmp/b" | head -n 1)
[ -n "$build" ] || fail "the shell names no /-/<build>/js/main.js: $(cat "$tmp/b")"
echo "PASS tinhub web: page URLs answer the shell with its policy (build $build)"

# assets: immutable by build, validators, gzip, and a stale build revalidated rather than cached for good
head_ "$base/-/$build/js/main.js"
has '^HTTP/1.1 200' main.js
has '^content-type: text/javascript' main.js
has '^cache-control: public, max-age=31536000, immutable' main.js
etag=$(sed -n 's/^[Ee][Tt][Aa][Gg]: *\(.*\)\r$/\1/p' "$tmp/h")
[ -n "$etag" ] || fail "main.js has no ETag"
head_ -H "If-None-Match: $etag" "$base/-/$build/js/main.js"
has '^HTTP/1.1 304' "main.js revalidated"
head_ -H 'Accept-Encoding: gzip' "$base/-/$build/css/app.css"
has '^content-encoding: gzip' app.css
gunzip -t "$tmp/b" 2> /dev/null || fail "app.css is not gzip"
head_ "$base/-/000000000000/js/main.js"
has '^HTTP/1.1 200' "an old build"
grep -qi '^cache-control: .*immutable' "$tmp/h" && fail "an old build's file is cached for good"
for path in "/-/$build/js/nope.js" "/-/$build/../web.tin" "/-/$build/.hidden"; do
	code=$(curl -s -o /dev/null -w '%{http_code}' --path-as-is "$base$path")
	[ "$code" = 404 ] || fail "GET $path: $code, not 404"
done
code=$(curl -s -o "$tmp/b" -w '%{http_code}' "$base/api/v1/no/such/route")
[ "$code" = 404 ] && jq -e '.code == "not_found"' "$tmp/b" > /dev/null || fail "an unknown API route: $code $(cat "$tmp/b")"
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$base/ada/tin")
[ "$code" = 404 ] || [ "$code" = 405 ] || fail "POST to a page URL: $code"
echo "PASS tinhub web: assets by build with ETag, 304 and gzip; bad paths 404"

if command -v node > /dev/null 2>&1; then
	node --test "$root"/products/tinhub/web/test/*.test.js > "$tmp/node.out" 2>&1 || { cat "$tmp/node.out"; fail "the UI's unit tests"; }
	echo "PASS tinhub web: the UI's unit tests ($(sed -n 's/^# pass //p' "$tmp/node.out") passed)"
else
	echo "SKIP tinhub web: the UI's unit tests (no node)"
fi
