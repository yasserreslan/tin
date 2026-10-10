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
# benchmark history: a few commits, each recording two benchmarks with tit bench record under .bench/
mkdir -p .bench
i=0
for v in "212 41.8" "208 41.2" "214 40.9" "196 38.4" "198 38.1" "187 37.6"; do
	i=$((i + 1))
	set -- $v
	as ada bench record fib "$1" --unit ms > /dev/null 2>&1 || fail "tit bench record"
	as ada bench record parse_json "$2" --unit us > /dev/null 2>&1 || fail "tit bench record"
	cp .tit/bench/fib.jsonl .tit/bench/parse_json.jsonl .bench/
	printf 'run %s\n' "$i" > .bench/RUNS
	as ada add .bench > /dev/null
	as ada commit -m "bench: run $i on the Linux box" > /dev/null
done
as ada push > "$dir/push.out" 2>&1 || fail "push: $(cat "$dir/push.out")"
as ada ship v0.1.0 > "$dir/ship.out" 2>&1 || fail "ship: $(cat "$dir/ship.out")"

# bob's stack on ada/tin: two changes on a branch, pushed for review; the top one amended and pushed again
cd "$dir"
(as bob clone "$base/ada/tin" tin-bob > "$dir/clone.out" 2>&1) || fail "bob's clone: $(cat "$dir/clone.out")"
cd "$dir/tin-bob"
as bob switch -c anvil-stream > /dev/null
printf '\n## Streaming\n\nLarge bodies stream from the pack.\n' >> README.md
as bob commit -a -m "README: say how large bodies stream" > /dev/null
f=products/tit/main.tin
sed 's/^\/\/ tit: version control written in Tin/\/\/ tit: version control, written in Tin,/' "$f" > "$f.new" && mv "$f.new" "$f"
printf '\n// banner is what tit prints first.\nfn banner() str {\n\treturn "tit"\n}\n' >> "$f"
as bob commit -a -m "tit: a banner for the first line" > /dev/null
as bob push > "$dir/push.out" 2>&1 || fail "bob's push: $(cat "$dir/push.out")"
printf '\n// version is the build'"'"'s version.\nfn version() str {\n\treturn "1"\n}\n' >> "$f"
as bob commit -a --amend -m "tit: a banner and a version" > /dev/null
as bob sync > "$dir/push.out" 2>&1 || fail "bob's second push: $(cat "$dir/push.out")"

# tinlang/tinhub: one commit
cd "$dir"
(as ada clone "$base/tinlang/tinhub" tinhub > "$dir/clone.out" 2>&1) || fail "clone tinhub: $(cat "$dir/clone.out")"
cd "$dir/tinhub"
cp "$root/products/tinhub/README.md" README.md
cp "$root/products/tinhub/main.tin" main.tin
as ada add README.md main.tin > /dev/null
as ada commit -m "tinhub: the binary" > /dev/null
as ada push > "$dir/push.out" 2>&1 || fail "push tinhub: $(cat "$dir/push.out")"
# replay failure groups for ada/tin, written straight into the database (development only: real capsules come sealed
# from a deployed server through tit replay push); skipped without psql and the TINHUB_DB_* environment
if command -v psql > /dev/null 2>&1 && [ -n "${TINHUB_DB_ADDR:-}" ]; then
	PGPASSWORD=${TINHUB_DB_PASSWORD:-} psql -q -h "${TINHUB_DB_ADDR%:*}" -p "${TINHUB_DB_ADDR##*:}" -U "${TINHUB_DB_USER:-tin}" "${TINHUB_DB_NAME:-tinhub}" > /dev/null <<-'SQL' || echo "seed: no replay rows" >&2
	WITH r AS (SELECT r.id FROM repos r JOIN owners o ON o.id = r.owner_id WHERE o.name = 'ada' AND r.name = 'tin')
	INSERT INTO replay_groups (repo_id, id, panic, route, state, fixed_by, count, first_at, last_at)
	SELECT r.id, g.id, g.panic, g.route, g.state, g.fixed, g.n, now() - g.first * interval '1 hour', now() - g.last * interval '1 minute' FROM r, (VALUES
		('9f2c1a7e40b1', 'index out of range [3] with length 3', 'POST /api/v1/carts/{id}/items', 'open', '', 41, 30, 4),
		('a07d55c3e9f0', 'nil map write in checkout.applyCoupon', 'POST /api/v1/checkout', 'open', '', 7, 6, 38),
		('3be0c4d12a88', 'division by zero in pricing.perUnit', 'GET /api/v1/quote', 'closed', 'opylwpwyxsztonmnnyzxmvknnmortstt', 3, 72, 900)
	) AS g(id, panic, route, state, fixed, n, first, last)
	ON CONFLICT DO NOTHING;
	WITH r AS (SELECT r.id FROM repos r JOIN owners o ON o.id = r.owner_id WHERE o.name = 'ada' AND r.name = 'tin')
	INSERT INTO capsules (repo_id, id, group_id, name, signer, status, route, size_bytes, created_at, expires_at)
	SELECT r.id, encode(sha256(convert_to(g || n::text, 'UTF8')), 'hex'), g, 'checkout', 'prod-eu-1', 500, rt, 2048 + n * 311, now() - n * interval '7 minutes', now() + interval '30 days' FROM r, (VALUES
		('9f2c1a7e40b1', 'POST /api/v1/carts/c_81/items'), ('a07d55c3e9f0', 'POST /api/v1/checkout')
	) AS x(g, rt), generate_series(1, 4) AS n
	ON CONFLICT DO NOTHING;
	SQL
fi

# push_fixes REPO DIR MESSAGE: tinlang/REPO gets DIR/main.tin on main (committed with MESSAGE), and a branch fix/NAME for
# each DIR/branches/fix-NAME.tin, whose commit message is the file's own "// fix/NAME: ..." comment. Leaves $dir/REPO on
# main.
push_fixes() {
	cd "$dir"
	(as ada clone "$base/tinlang/$1" "$1" > "$dir/clone.out" 2>&1) || fail "clone $1: $(cat "$dir/clone.out")"
	cd "$dir/$1"
	cp "$2/main.tin" main.tin
	as ada add main.tin > /dev/null
	as ada commit -m "$3" > /dev/null
	as ada push > "$dir/push.out" 2>&1 || fail "push $1: $(cat "$dir/push.out")"
	for f in "$2"/branches/fix-*.tin; do
		name=fix/$(basename "$f" .tin | sed 's/^fix-//')
		msg=$(awk -v n="// $name: " 'index($0, n) == 1 { on = 1 } on && !/^\/\/ / { exit } on { print substr($0, 4) }' "$f")
		as ada switch main > /dev/null 2>&1
		as ada switch -c "$name" > /dev/null
		cp "$f" main.tin
		as ada commit -a -m "$msg" > /dev/null
		as ada push > "$dir/push.out" 2>&1 || fail "push $1 $name: $(cat "$dir/push.out")"
	done
	as ada switch main > /dev/null 2>&1
}
# check_all REPO DIR: a check of main and of each fix branch push_fixes made from DIR, against each of tinlang/REPO's
# open failure groups.
check_all() {
	for g in $(api ada GET "/api/v1/repos/tinlang/$1/replay/groups?state=open" | jq -r '.groups[].id'); do
		for b in main $(for f in "$2"/branches/fix-*.tin; do basename "$f" .tin | sed 's#^fix-#fix/#'; done); do
			api ada POST "/api/v1/repos/tinlang/$1/replay/checks" "{\"target\":\"$b\",\"group\":\"$g\"}" > /dev/null || fail "check $1 $b"
		done
	done
}

# tinlang/shop: replay checks (design/tinhub.md §12.1) with real capsules. dev/shop's checkout service is recorded
# failing twice (a panic on a declined cart, a 500 while its payment service is down), sealed for the node's runner key,
# and pushed with five fix branches; each branch and main is then checked against both failures, so the shop's Replay
# page shows a fix that passes, three ways to diverge and a fix that still panics, each with its explanation. Needs the
# runner (runner.key and runner.tin set, the runner role; runner.sandbox = off where the sandbox cannot run, such as
# macOS), a Tin compiler ($SEED_TINC, default bin/tinc) and Redis (redis-server, or $SEED_REDIS_ADDR); skipped without.
seed_checks() {
	tinc=${SEED_TINC:-$root/bin/tinc}
	[ -x "$tinc" ] || { echo "seed: no replay checks: no compiler at $tinc (set SEED_TINC)" >&2; return 0; }
	api ada POST /api/v1/repos '{"owner":"tinlang","name":"shop","visibility":"private","description":"A checkout service, its production failures and the fixes tried for them"}' > /dev/null
	pub=$(api ada PUT /api/v1/orgs/tinlang/runner '{}' 2> /dev/null | jq -r '.public_key // empty' || true)
	[ -n "$pub" ] || { echo "seed: no replay checks: no runner has registered a key (run tinhub with the runner role and runner.key)" >&2; return 0; }
	redis=${SEED_REDIS_ADDR:-}
	rpid=
	if [ -z "$redis" ]; then
		command -v redis-server > /dev/null 2>&1 || { echo "seed: no replay checks: no redis-server (or SEED_REDIS_ADDR) to record with" >&2; return 0; }
		rport=16399
		while redis-cli -p $rport ping > /dev/null 2>&1; do rport=$((rport + 1)); done
		redis-server --port $rport --bind 127.0.0.1 --save '' --appendonly no > "$dir/redis.log" 2>&1 &
		rpid=$!
		redis=127.0.0.1:$rport
		for i in $(seq 1 50); do redis-cli -p $rport ping > /dev/null 2>&1 && break; sleep 0.1; done
	fi
	shop=$root/products/tinhub/dev/shop
	pay_port=${SEED_PAYMENTS_PORT:-9197}
	api ada PUT /api/v1/repos/tinlang/shop/runner "{\"entry\":\"main.tin\",\"sample\":20,\"env\":[\"PAYMENTS_URL=http://127.0.0.1:$pay_port\"]}" > /dev/null
	push_fixes shop "$shop" "shop: checkout reads the receipt the payment service answers"
	signing=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')
	REDIS_ADDR=$redis TIN_REPLAY_RECIPIENTS=$pub TIN_REPLAY_SIGNING_KEY=$signing PAYMENTS_PORT=$pay_port \
		sh "$shop/record.sh" "$tinc" "$dir/spool" > "$dir/record.out" 2>&1 || fail "recording shop: $(cat "$dir/record.out")"
	as ada replay push "$dir/spool" --commit main > "$dir/replay.out" 2>&1 || fail "tit replay push: $(cat "$dir/replay.out")"
	check_all shop "$shop"
	echo "seed: tinlang/shop has two failures and checks of six revisions against them: $base/tinlang/shop/replay (the runner works through them)"
}
# tinlang/orders: replay checks on Postgres, Redis and a real website (dev/orders/README.md). Its payment service reads an
# order from Postgres, an exchange rate from Redis or api.frankfurter.dev, writes the payment and answers the card fee's
# share; it is recorded failing three ways (an unpriced currency, a double click, a free order) and checked at main and
# eight fix branches. It calls the internet, so it runs only with SEED_ORDERS=1, and needs psql and a Postgres database
# it may empty: SEED_ORDERS_PG_ADDR, SEED_ORDERS_PG_USER, SEED_ORDERS_PG_PASSWORD and SEED_ORDERS_PG_DATABASE.
seed_orders() {
	[ "${SEED_ORDERS:-}" = 1 ] || return 0
	[ -n "${pub:-}" ] && [ -n "${redis:-}" ] || { echo "seed: no tinlang/orders: replay checks were skipped" >&2; return 0; }
	[ -n "${SEED_ORDERS_PG_ADDR:-}" ] && [ -n "${SEED_ORDERS_PG_DATABASE:-}" ] || { echo "seed: no tinlang/orders: set SEED_ORDERS_PG_ADDR, _USER, _PASSWORD and _DATABASE" >&2; return 0; }
	orders=$root/products/tinhub/dev/orders
	pg="\"POSTGRES_ADDR=$SEED_ORDERS_PG_ADDR\",\"POSTGRES_USER=${SEED_ORDERS_PG_USER:-}\",\"POSTGRES_PASSWORD=${SEED_ORDERS_PG_PASSWORD:-}\",\"POSTGRES_DATABASE=$SEED_ORDERS_PG_DATABASE\""
	api ada POST /api/v1/repos '{"owner":"tinlang","name":"orders","visibility":"private","description":"A payment service on Postgres, Redis and a rates website, its failures and the fixes tried for them"}' > /dev/null
	# a replay never connects, but the service reads its settings before it serves
	api ada PUT /api/v1/repos/tinlang/orders/runner "{\"entry\":\"main.tin\",\"sample\":20,\"env\":[$pg,\"REDIS_ADDR=$redis\"]}" > /dev/null
	push_fixes orders "$orders" "orders: charge an order in US dollars at today's rate"
	signing=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')
	POSTGRES_ADDR=$SEED_ORDERS_PG_ADDR POSTGRES_USER=${SEED_ORDERS_PG_USER:-} POSTGRES_PASSWORD=${SEED_ORDERS_PG_PASSWORD:-} \
		POSTGRES_DATABASE=$SEED_ORDERS_PG_DATABASE REDIS_ADDR=$redis TIN_REPLAY_RECIPIENTS=$pub TIN_REPLAY_SIGNING_KEY=$signing \
		sh "$orders/record.sh" "$tinc" "$dir/orders-spool" > "$dir/record.out" 2>&1 || fail "recording orders: $(cat "$dir/record.out")"
	as ada replay push "$dir/orders-spool" --commit main > "$dir/replay.out" 2>&1 || fail "tit replay push orders: $(cat "$dir/replay.out")"
	check_all orders "$orders"
	echo "seed: tinlang/orders has three failures and checks of nine revisions against them: $base/tinlang/orders/replay"
}
seed_checks
seed_orders
[ -n "${rpid:-}" ] && kill "$rpid" 2>/dev/null || true
echo "seeded $base: ada (site admin) and bob; sessions in $dir/ada.cookie and $dir/bob.cookie"
