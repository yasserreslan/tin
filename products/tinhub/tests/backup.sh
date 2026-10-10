#!/bin/sh
# The restore drill (#1024) against TINHUB_TEST_DB, on the directory store and, with TINHUB_TEST_S3 (the fake S3, or
# MinIO), on a bucket: the single-instance shape, content in object storage. Usage: backup.sh TINHUB REPO_DRIVER
#   deploy/backup.sh copies the database and the pack store's objects; a second run copies no object again; with the
#   objects and the schema gone (on a bucket: a new, empty bucket), deploy/restore.sh brings back the same refs and the
#   same live packs (tinhub check, before and after), and tinhub check names a pack that is missing.
set -eu
hub=$1
driver=$2
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
deploy=$(cd "$(dirname "$0")/../deploy" && pwd)
fail() {
	echo "FAIL tinhub backup ($store): $*"
	exit 1
}
dbaddr=${TINHUB_TEST_DB%%/*}
export TINHUB_DB_ADDR="$dbaddr" TINHUB_DB_NAME="${TINHUB_TEST_DB#*/}" TINHUB_DB_USER="${TINHUB_TEST_DB_USER:-}" TINHUB_DB_PASSWORD="${TINHUB_TEST_DB_PASSWORD:-}"
export TINHUB_SECRETS_NONCE=test-nonce-secret-0123456789 TINHUB_SECRETS_COOKIE=test-cookie-key-0123456789
export TINHUB_BIN="$hub"

# drill STORE: the drill on the dir or s3 store
drill() {
	store=$1
	w="$tmp/$store"
	mkdir -p "$w"
	"$driver" setup "$w/packs" > /dev/null
	"$driver" crash "$w/packs" > /dev/null 2>&1 || true
	export TINHUB_PACKS_STORE=dir TINHUB_PACKS_DIR="$w/packs"
	if [ "$store" = s3 ]; then
		# the driver writes to disk; packs copy puts it in the bucket, and from here on the bucket is the only copy
		export TINHUB_S3_ENDPOINT="$TINHUB_TEST_S3" TINHUB_S3_ACCESS_KEY="$TINHUB_TEST_S3_KEY" TINHUB_S3_SECRET_KEY="$TINHUB_TEST_S3_SECRET"
		export TINHUB_S3_REGION=us-east-1 TINHUB_S3_BUCKET="tinhub-backup-$$"
		"$hub" packs copy > "$w/copy" 2>&1 || fail "packs copy: $(cat "$w/copy")"
		rm -rf "$w/packs"
		export TINHUB_PACKS_STORE=s3 TINHUB_PACKS_DIR="$w/node"
	fi
	"$hub" check > "$w/before" || fail "tinhub check before the backup: $(cat "$w/before")"
	grep -q '^packs [0-9a-f]\{32\} ([1-9]' "$w/before" || fail "no live packs to back up: $(cat "$w/before")"
	sh "$deploy/backup.sh" "$w/backup" > "$w/b1" || fail "backup.sh: $(cat "$w/b1")"
	grep -q ', [1-9][0-9]* new objects$' "$w/b1" || fail "the first backup copied no objects: $(cat "$w/b1")"
	sleep 1
	sh "$deploy/backup.sh" "$w/backup" > "$w/b2" || fail "backup.sh again: $(cat "$w/b2")"
	grep -q ', 0 new objects$' "$w/b2" || fail "the second backup copied objects again: $(cat "$w/b2")"
	[ "$(ls "$w/backup/db" | wc -l | tr -d ' ')" = 2 ] || fail "two dumps expected: $(ls "$w/backup/db")"
	echo "PASS tinhub backup ($store): a dump and the store's objects, incrementally"

	# the server is lost: no objects (on a bucket, a new empty one and a new node), an empty schema
	rm -rf "$w/packs" "$w/node"
	if [ "$store" = s3 ]; then
		export TINHUB_S3_BUCKET="tinhub-restored-$$" TINHUB_PACKS_DIR="$w/node2"
	fi
	"$driver" setup "$w/other" > /dev/null
	sh "$deploy/restore.sh" "$w/backup" > "$w/r" || fail "restore.sh: $(cat "$w/r")"
	grep -q ', [1-9][0-9]* objects copied back$' "$w/r" || fail "the restore copied nothing back: $(cat "$w/r")"
	"$hub" check > "$w/after" || fail "tinhub check after the restore: $(cat "$w/after")"
	cmp -s "$w/before" "$w/after" || fail "the restore differs: $(diff "$w/before" "$w/after")"
	echo "PASS tinhub restore ($store): the same refs and live packs"
}

drill dir
# a lost pack is named
victim=$(find "$tmp/dir/packs/repos" -name '*.pack' | head -n 1)
rm "$victim"
if "$hub" check > "$tmp/broken"; then
	fail "tinhub check passed with a pack missing"
fi
grep -q "^missing repos/.*$(basename "$victim")" "$tmp/broken" || fail "the missing pack is not named: $(cat "$tmp/broken")"
echo "PASS tinhub check: a missing pack is named"
if [ -n "${TINHUB_TEST_S3:-}" ]; then
	drill s3
else
	echo "SKIP tinhub backup on a bucket (TINHUB_TEST_S3 is not set)"
fi
