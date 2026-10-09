#!/bin/sh
# backup.sh TARGET: the nightly backup of a phase 1 tinhub (deploy/RUNBOOK.md). A pg_dump of the database into
# TARGET/db, then every pack file under packs.dir that TARGET/packs lacks. The dump is taken first: a pack the dump
# names live was in the store before its push committed, and packs retired since stay for packs.grace, so every
# pack the dump needs is copied. The last TINHUB_BACKUP_KEEP dumps are kept (14 by default); packs are never deleted
# from the target.
set -eu
target=${1:?usage: backup.sh TARGET}
. "$(dirname "$0")/lib.sh"
pg_env
mkdir -p "$target/db" "$target/packs"
stamp=$(date -u +%Y%m%dT%H%M%SZ)
pg_dump --format=custom --no-owner --file="$target/db/$stamp.dump.partial"
mv "$target/db/$stamp.dump.partial" "$target/db/$stamp.dump"
n=$(copy_new "$packs" "$target/packs")
keep=${TINHUB_BACKUP_KEEP:-14}
ls "$target/db" | grep '\.dump$' | sort -r | tail -n +$((keep + 1)) | while read -r old; do rm -f "$target/db/$old"; done
echo "tinhub backup: $target/db/$stamp.dump, $n new pack files"
