#!/bin/sh
# The restore drill (#1024) against TINHUB_TEST_DB. Usage: backup.sh TINHUB REPO_DRIVER
#   deploy/backup.sh copies the database and the packs; a second run copies no pack again; with the packs and the
#   schema gone, deploy/restore.sh brings back the same refs and the same live packs (tinhub check, before and after),
#   and tinhub check names a pack that is missing.
set -eu
hub=$1
driver=$2
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
deploy=$(cd "$(dirname "$0")/../deploy" && pwd)
fail() {
	echo "FAIL tinhub backup: $*"
	exit 1
}
"$driver" setup "$tmp/packs" > /dev/null
"$driver" crash "$tmp/packs" > /dev/null 2>&1 || true
dbaddr=${TINHUB_TEST_DB%%/*}
export TINHUB_DB_ADDR="$dbaddr" TINHUB_DB_NAME="${TINHUB_TEST_DB#*/}" TINHUB_DB_USER="${TINHUB_TEST_DB_USER:-}" TINHUB_DB_PASSWORD="${TINHUB_TEST_DB_PASSWORD:-}"
export TINHUB_SECRETS_NONCE=test-nonce-secret-0123456789 TINHUB_SECRETS_COOKIE=test-cookie-key-0123456789
export TINHUB_PACKS_DIR="$tmp/packs"
"$hub" check > "$tmp/before" || fail "tinhub check before the backup: $(cat "$tmp/before")"
grep -q '^packs [0-9a-f]\{32\} ([1-9]' "$tmp/before" || fail "no live packs to back up: $(cat "$tmp/before")"
sh "$deploy/backup.sh" "$tmp/backup" > "$tmp/b1" || fail "backup.sh: $(cat "$tmp/b1")"
grep -q ', [1-9][0-9]* new pack files$' "$tmp/b1" || fail "the first backup copied no packs: $(cat "$tmp/b1")"
sleep 1
sh "$deploy/backup.sh" "$tmp/backup" > "$tmp/b2" || fail "backup.sh again: $(cat "$tmp/b2")"
grep -q ', 0 new pack files$' "$tmp/b2" || fail "the second backup copied packs again: $(cat "$tmp/b2")"
[ "$(ls "$tmp/backup/db" | wc -l | tr -d ' ')" = 2 ] || fail "two dumps expected: $(ls "$tmp/backup/db")"
echo "PASS tinhub backup: a dump and the packs, incrementally"

# the server is lost: no packs, an empty schema
rm -rf "$tmp/packs"
"$driver" setup "$tmp/other" > /dev/null
sh "$deploy/restore.sh" "$tmp/backup" > "$tmp/r" || fail "restore.sh: $(cat "$tmp/r")"
"$hub" check > "$tmp/after" || fail "tinhub check after the restore: $(cat "$tmp/after")"
cmp -s "$tmp/before" "$tmp/after" || fail "the restore differs: $(diff "$tmp/before" "$tmp/after")"
echo "PASS tinhub restore: the same refs and live packs"

# a lost pack is named
victim=$(find "$tmp/packs/repos" -name '*.pack' | head -n 1)
rm "$victim"
if "$hub" check > "$tmp/broken"; then
	fail "tinhub check passed with a pack missing"
fi
grep -q "^missing repos/.*$(basename "$victim")" "$tmp/broken" || fail "the missing pack is not named: $(cat "$tmp/broken")"
echo "PASS tinhub check: a missing pack is named"
