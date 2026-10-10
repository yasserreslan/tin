# Shared by backup.sh and restore.sh: the Postgres connection from the same TINHUB_DB_* variables tinhub reads
# (a value of the form file:PATH is read from PATH), and the packs directory.
pg_env() {
	addr=${TINHUB_DB_ADDR:-127.0.0.1:5432}
	export PGHOST="${addr%:*}" PGPORT="${addr##*:}" PGDATABASE="${TINHUB_DB_NAME:-tinhub}" PGUSER="${TINHUB_DB_USER:-tinhub}"
	pw=${TINHUB_DB_PASSWORD:-}
	case "$pw" in
	file:*) pw=$(cat "${pw#file:}") ;;
	esac
	[ -n "$pw" ] && export PGPASSWORD="$pw"
	packs=${TINHUB_PACKS_DIR:-/var/lib/tinhub}
	packs=${packs%/}
}

# copy_new FROM TO: every file under FROM/repos that TO/repos lacks, through a temporary name and a rename, so an
# interrupted copy leaves no partial pack under its real name. Packs never change once written, so a file already
# there is the same file, and nothing is ever deleted from TO.
copy_new() {
	[ -d "$1/repos" ] || return 0
	n=0
	for f in $(cd "$1" && find repos -type f ! -name '.*' | sort); do
		[ -e "$2/$f" ] && continue
		mkdir -p "$2/$(dirname "$f")"
		cp "$1/$f" "$2/$f.partial"
		mv "$2/$f.partial" "$2/$f"
		n=$((n + 1))
	done
	echo "$n"
}
