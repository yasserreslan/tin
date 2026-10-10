# Shared by backup.sh and restore.sh: the Postgres connection from the same TINHUB_DB_* variables tinhub reads
# (a value of the form file:PATH is read from PATH). The pack store is reached through the tinhub binary
# (TINHUB_BIN, tinhub on the PATH by default), which reads its own configuration (/etc/tinhub/tinhub.conf).
pg_env() {
	addr=${TINHUB_DB_ADDR:-127.0.0.1:5432}
	export PGHOST="${addr%:*}" PGPORT="${addr##*:}" PGDATABASE="${TINHUB_DB_NAME:-tinhub}" PGUSER="${TINHUB_DB_USER:-tinhub}"
	pw=${TINHUB_DB_PASSWORD:-}
	case "$pw" in
	file:*) pw=$(cat "${pw#file:}") ;;
	esac
	[ -n "$pw" ] && export PGPASSWORD="$pw"
	return 0
}

# copied OUTPUT: the object count in tinhub packs backup's or restore's line ("tinhub: copied N objects ...").
copied() {
	n=$(printf '%s\n' "$1" | sed -n 's/^tinhub: copied \([0-9][0-9]*\) objects.*/\1/p')
	[ -n "$n" ] || { echo "unexpected output from tinhub packs: $1" >&2; return 1; }
	echo "$n"
}
