#!/bin/sh
# restore.sh TARGET [DUMP]: restores the newest dump in TARGET/db (or DUMP) into the database the TINHUB_DB_*
# variables name, replacing what is there, and copies the objects the pack store lacks from TARGET/packs back into it
# (tinhub packs restore: the bucket, or packs.dir in development). Stop tinhub first; run tinhub check after
# (deploy/RUNBOOK.md).
set -eu
target=${1:?usage: restore.sh TARGET [DUMP]}
. "$(dirname "$0")/lib.sh"
pg_env
dump=${2:-$(ls "$target/db" | grep '\.dump$' | sort | tail -n 1)}
[ -n "$dump" ] || { echo "restore.sh: no dump in $target/db" >&2; exit 1; }
case "$dump" in
/*) ;;
*) dump="$target/db/$dump" ;;
esac
PGOPTIONS='-c client_min_messages=warning' psql -q -v ON_ERROR_STOP=1 -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;' > /dev/null
pg_restore --no-owner --exit-on-error --dbname="$PGDATABASE" "$dump"
n=$(copied "$("${TINHUB_BIN:-tinhub}" packs restore "$target/packs")")
echo "tinhub restore: $dump, $n objects copied back"
