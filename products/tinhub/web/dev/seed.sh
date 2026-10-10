#!/bin/sh
# Seeds a running tinhub node with people, an org and repositories to look at in the web UI (#1081): ada (a site
# admin) and bob, each with a key and a browser session; the org tinlang with a team; the public repository ada/tin with a
# snapshot of this repository's tit and tinhub sources pushed in a few commits; a private repository; grants.
# Usage: seed.sh TINHUB TIT [BASE_URL]. TINHUB runs `admin invite` with the node's configuration (the TINHUB_*
# environment). Sessions land in $SEED_DIR (default /tmp/tinhub-seed): ada.jar and bob.jar are curl cookie jars, and
# ada.cookie the session cookie's value, for a browser.
set -eu
hub=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
tit=$(cd "$(dirname "$2")" && pwd)/$(basename "$2")
base=${3:-${TINHUB_PUBLIC_URL:-http://127.0.0.1:8080}}
dir=${SEED_DIR:-/tmp/tinhub-seed}
root=$(cd "$(dirname "$0")/../../../.." && pwd)
rm -rf "$dir"
mkdir -p "$dir"
export TIT_NO_PAGER=1
fail() {
	echo "seed: $*" >&2
	exit 1
}
as() {
	who=$1
	shift
	HOME="$dir/$who" XDG_CONFIG_HOME="$dir/$who" "$tit" "$@"
}
# person NAME EMAIL [--site-admin]: an account with a key, and a browser session in $dir/NAME.jar
person() {
	mkdir -p "$dir/$1"
	as "$1" config set --user user.name "$1" > /dev/null
	as "$1" config set --user user.email "$2" > /dev/null
	as "$1" key > /dev/null 2>&1 || true
	code=$("$hub" admin invite "$2" ${3:-} | sed -n "s/^invite for $2: \(.*\)\$/\1/p")
	[ -n "$code" ] || fail "no invite for $2"
	as "$1" key add "$base" "$code" > "$dir/$1.out" 2>&1 || fail "tit key add for $1: $(cat "$dir/$1.out")"
	login=$(curl -sf -c "$dir/$1.jar" -X POST "$base/tit/v1/login" | jq -r .code)
	mkdir -p "$dir/$1/ws"
	(cd "$dir/$1/ws" && as "$1" init > /dev/null 2>&1 && as "$1" login "$base" --code "$login") > "$dir/$1.out" 2>&1 || fail "tit login for $1: $(cat "$dir/$1.out")"
	curl -sf -b "$dir/$1.jar" -c "$dir/$1.jar" -X POST "$base/tit/v1/login/$login/session" > /dev/null || fail "the session of $1"
	awk '$6 == "tinhub_session" { print $7 }' "$dir/$1.jar" > "$dir/$1.cookie"
}
# api WHO METHOD PATH [BODY]
api() {
	if [ $# -ge 4 ]; then
		curl -sf -b "$dir/$1.jar" -X "$2" -H 'Content-Type: application/json' --data "$4" "$base$3"
	else
		curl -sf -b "$dir/$1.jar" -X "$2" "$base$3"
	fi
}
person ada ada@example.com --site-admin
person bob bob@example.com
api ada PATCH /api/v1/user '{"display":"Ada Lovelace"}' > /dev/null
api bob PATCH /api/v1/user '{"display":"Bob Ross"}' > /dev/null
api ada POST /api/v1/orgs '{"name":"tinlang","display":"The Tin language"}' > /dev/null
api ada PUT /api/v1/orgs/tinlang/members/bob '{"role":"member"}' > /dev/null
api ada POST /api/v1/orgs/tinlang/teams '{"name":"core"}' > /dev/null
api ada PUT /api/v1/orgs/tinlang/teams/core/members/bob > /dev/null
api ada POST /api/v1/repos '{"name":"tin","visibility":"public","description":"A compiled, self-hosted language for servers and tools"}' > /dev/null
api ada POST /api/v1/repos '{"owner":"tinlang","name":"tinhub","visibility":"public","description":"Hosting for tit repositories, written in Tin"}' > /dev/null
api ada POST /api/v1/repos '{"name":"notes","visibility":"private","description":"Private notes"}' > /dev/null
api ada PUT /api/v1/repos/ada/tin/collaborators/bob '{"role":"write"}' > /dev/null
api ada PUT /api/v1/repos/tinlang/tinhub/teams/core '{"role":"write"}' > /dev/null

# ada/tin: a few commits of real sources
(cd "$dir" && as ada clone "$base/ada/tin" tin > "$dir/clone.out" 2>&1) || fail "clone: $(cat "$dir/clone.out")"
cd "$dir/tin"
cp "$root/README.md" README.md
mkdir -p products
cp -R "$root/products/tit" products/tit
rm -rf products/tit/tests
as ada add README.md products > /dev/null
as ada commit -m "tit: the version control system

The first snapshot of tit's sources." > /dev/null
mkdir -p products/tinhub
for d in accounts api web; do cp -R "$root/products/tinhub/$d" "products/tinhub/$d"; done
cp "$root/products/tinhub/README.md" products/tinhub/README.md
as ada add products > /dev/null
as ada commit -m "tinhub: accounts, the API and the web pages" > /dev/null
printf '\nSee products/tinhub for the hosting service.\n' >> README.md
as ada add README.md > /dev/null
as bob config set --user user.name bob > /dev/null 2>&1 || true
as ada commit -m "README: point at tinhub" > /dev/null
as ada push > "$dir/push.out" 2>&1 || fail "push: $(cat "$dir/push.out")"
as ada tag v0.1.0 -m "v0.1.0" > /dev/null 2>&1 || true
as ada push --tags > /dev/null 2>&1 || true

# tinlang/tinhub: one commit
cd "$dir"
(as ada clone "$base/tinlang/tinhub" tinhub > "$dir/clone.out" 2>&1) || fail "clone tinhub: $(cat "$dir/clone.out")"
cd "$dir/tinhub"
cp "$root/products/tinhub/README.md" README.md
cp "$root/products/tinhub/main.tin" main.tin
as ada add README.md main.tin > /dev/null
as ada commit -m "tinhub: the binary" > /dev/null
as ada push > "$dir/push.out" 2>&1 || fail "push tinhub: $(cat "$dir/push.out")"
echo "seeded $base: ada (site admin) and bob; sessions in $dir/ada.cookie and $dir/bob.cookie"
