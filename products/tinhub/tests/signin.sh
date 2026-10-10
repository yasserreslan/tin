#!/bin/sh
# Accounts end to end (#1012, #1018) against TINHUB_TEST_DB. Usage: signin.sh TINHUB REPO_DRIVER TIT
#   - tinhub admin invite makes a code; tit key add registers bob's key with it, once;
#   - bob, who can read the public ada/tin, is refused a push; granted write, his push lands and the API shows it;
#   - a 50 MB blob streams from the API, and 100 concurrent clones finish with the server's peak memory bounded;
#   - a browser asks for a login request, bob approves it with tit login, the browser claims it and gets a session
#     that reads the repository once it is private (anonymous callers get 404); signing out ends it.
#   - deploy/mirror-sync.sh keeps a repository a mirror of a git repository (adopt, then push with a mirror key).
#   - restarting the server three times while pushes run loses none of them.
set -eu
hub=$1
driver=$2
tit=$3
tmp=$(mktemp -d)
deploy=$(cd "$(dirname "$0")/../deploy" && pwd)
pid=
. "$(dirname "$0")/lib.sh"
cleanup() {
	[ -n "$pid" ] && kill "$pid" 2>/dev/null || true
	stop_redis
	rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
fail() {
	echo "FAIL tinhub sign-in: $*"
	[ -f "$tmp/hub.log" ] && tail -n 20 "$tmp/hub.log"
	exit 1
}
start_redis || fail "sessions need Redis (redis-server, or docker)"
"$driver" setup "$tmp/node" > /dev/null
dbaddr=${TINHUB_TEST_DB%%/*}
export TINHUB_DB_ADDR="$dbaddr" TINHUB_DB_NAME="${TINHUB_TEST_DB#*/}" TINHUB_DB_USER="${TINHUB_TEST_DB_USER:-}" TINHUB_DB_PASSWORD="${TINHUB_TEST_DB_PASSWORD:-}"
export TINHUB_SECRETS_NONCE=test-nonce-secret-0123456789 TINHUB_SECRETS_COOKIE=test-cookie-key-0123456789
export TINHUB_LISTEN=127.0.0.1:18434 TINHUB_PACKS_DIR="$tmp/node" TINHUB_PACKS_SWEEP=0 TINHUB_REDIS_ADDR="127.0.0.1:$port"
base=http://$TINHUB_LISTEN
export TINHUB_PUBLIC_URL="$base"
start() {
	"$hub" run >> "$tmp/hub.log" 2>&1 &
	pid=$!
	up=0
	for i in $(seq 1 40); do
		curl -sf "$base/healthz" > /dev/null 2>&1 && { up=1; break; }
		sleep 0.25
	done
	[ $up = 1 ] || fail "the server did not start"
}
start

export TIT_NO_PAGER=1
bob() {
	HOME="$tmp/bob" XDG_CONFIG_HOME="$tmp/bob" "$tit" "$@"
}
mkdir -p "$tmp/bob"
bob config set --user user.name Bob
bob config set --user user.email bob@example.com
bob key > /dev/null 2>&1

# an invite lets bob's key in, once
"$hub" admin invite bob@example.com > "$tmp/invite.out" || fail "tinhub admin invite: $(cat "$tmp/invite.out")"
code=$(sed -n 's/^invite for bob@example.com: \(.*\)$/\1/p' "$tmp/invite.out")
[ -n "$code" ] || fail "the invite: $(cat "$tmp/invite.out")"
bob key add "$base" "$code" > "$tmp/out" 2>&1 || fail "tit key add: $(cat "$tmp/out")"
grep -q "^Added the key of bob@example.com" "$tmp/out" || fail "tit key add said: $(cat "$tmp/out")"
if bob key add "$base" "$code" > "$tmp/out" 2>&1; then
	fail "an invite was used twice"
fi
echo "PASS tinhub sign-in: an invite adds a key, once"

# bob reads the public repository but cannot push to it until he is granted write
curl -sf "$base/api/v1/repos/ada/tin/refs" | jq -r ".refs[0].target" > "$tmp/main.before"
(cd "$tmp" && bob clone "$base/ada/tin" work > "$tmp/out" 2>&1) || fail "bob's clone: $(cat "$tmp/out")"
cd "$tmp/work"
printf 'two\n' > b.txt
bob add b.txt
bob commit -m second > /dev/null
if bob push > "$tmp/out" 2>&1; then
	fail "a reader pushed"
fi
grep -q "not push" "$tmp/out" || fail "the refused push said: $(cat "$tmp/out")"
"$driver" grant bob@example.com write
bob push > "$tmp/out" 2>&1 || fail "bob's push: $(cat "$tmp/out")"
cd "$tmp"
refs=$(curl -sf "$base/api/v1/repos/ada/tin/refs") || fail "GET refs after the push"
echo "$refs" | jq -e '.refs[0].name == "refs/heads/main"' > /dev/null || fail "refs: $refs"
before=$(cat "$tmp/main.before")
echo "$refs" | jq -e --arg b "$before" '.refs[0].target != $b' > /dev/null || fail "main did not move: $refs"
echo "PASS tinhub protocol: a reader's push is refused, a writer's lands"

# 100 concurrent clones, each fetch inside its budget (fetch memory): all finish, and the server's peak memory stays
# bounded. A fetch holds the objects it sends, so its memory is about the size of what it sends: this runs before the
# 50 MB file below is pushed.
hwm0=$(awk '/^VmHWM/ { print $2 }' "/proc/$pid/status" 2>/dev/null || echo 0)
i=0
clones=
while [ $i -lt 100 ]; do
	i=$((i + 1))
	(cd "$tmp" && bob clone "$base/ada/tin" "c$i" > "$tmp/c$i.out" 2>&1 && [ "$(cat "$tmp/c$i/b.txt")" = two ]) &
	clones="$clones $!"
done
bad=0
for c in $clones; do wait "$c" || bad=$((bad + 1)); done
[ $bad = 0 ] || fail "$bad of 100 concurrent clones failed: $(cat "$tmp/c1.out")"
if [ -r "/proc/$pid/status" ]; then
	hwm=$(awk '/^VmHWM/ { print $2 }' "/proc/$pid/status")
	echo "tinhub: peak memory $((hwm / 1024)) MiB after 100 concurrent clones (was $((hwm0 / 1024)) MiB)"
	[ "$hwm" -lt $((1024 * 1024)) ] || fail "peak memory $((hwm / 1024)) MiB over 1 GiB"
fi
rm -rf "$tmp"/c[0-9]*
echo "PASS tinhub protocol: 100 concurrent clones"

# a 50 MB blob pushed and read back through the API, streamed in pieces
cd "$tmp/work"
head -c 52428800 /dev/urandom > big.bin
bob add big.bin
bob commit -m "a big file" > /dev/null
bob push > "$tmp/out" 2>&1 || fail "the big push: $(cat "$tmp/out")"
cd "$tmp"
main=$(curl -sf "$base/api/v1/repos/ada/tin/refs" | jq -r '.refs[0].target')
curl -s "$base/api/v1/repos/ada/tin/commits/$main" > "$tmp/commit.json"
treeid=$(jq -r .tree "$tmp/commit.json")
[ ${#treeid} -eq 64 ] || fail "the commit: $(cat "$tmp/commit.json")"
blob=$(curl -sf "$base/api/v1/repos/ada/tin/trees/$treeid" | jq -r '.entries[] | select(.name == "big.bin") | .id')
[ ${#blob} -eq 64 ] || fail "big.bin is not in main's tree: $main $(curl -s "$base/api/v1/repos/ada/tin/trees/$treeid")"
curl -sf -o "$tmp/big.out" "$base/api/v1/repos/ada/tin/blobs/$blob" || fail "GET the 50 MB blob"
cmp -s "$tmp/big.out" "$tmp/work/big.bin" || fail "the 50 MB blob differs"
echo "PASS tinhub api: a 50 MB blob streams"

# a browser signs in with bob's key, and its session reads the private repository
"$driver" private
code=$(curl -s -o /dev/null -w '%{http_code}' "$base/api/v1/repos/ada/tin")
[ "$code" = 404 ] || fail "anonymous read of a private repository: $code"
curl -s -c "$tmp/jar" -X POST "$base/tit/v1/login" > "$tmp/req.json"
login=$(jq -r .code "$tmp/req.json")
[ ${#login} -eq 9 ] || fail "the login request: $(cat "$tmp/req.json")"
claim=$(curl -s -b "$tmp/jar" -o /dev/null -w '%{http_code}' -X POST "$base/tit/v1/login/$login/session")
[ "$claim" != 200 ] || fail "a pending login request was claimed"
(cd "$tmp/work" && bob login --code "$login") > "$tmp/out" 2>&1 || fail "tit login: $(cat "$tmp/out")"
curl -s "$base/tit/v1/login/$login" | jq -e '.state == "approved" and .email == "bob@example.com"' > /dev/null || fail "the login request is not approved"
claim=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$base/tit/v1/login/$login/session")
[ "$claim" != 200 ] || fail "a login request was claimed without its claim cookie"
curl -sf -b "$tmp/jar" -c "$tmp/jar" -X POST "$base/tit/v1/login/$login/session" > "$tmp/out" || fail "the claim: $(cat "$tmp/out")"
code=$(curl -s -b "$tmp/jar" -o "$tmp/out" -w '%{http_code}' "$base/api/v1/repos/ada/tin")
[ "$code" = 200 ] || fail "a signed-in read of the private repository: $code $(cat "$tmp/out")"
curl -sf -b "$tmp/jar" -c "$tmp/jar" -X POST "$base/tit/v1/logout" > /dev/null || fail "sign-out"
code=$(curl -s -b "$tmp/jar" -o /dev/null -w '%{http_code}' "$base/api/v1/repos/ada/tin")
[ "$code" = 404 ] || fail "a read after sign-out: $code"
echo "PASS tinhub sign-in: a browser session from tit login, and sign-out"

# a git repository mirrored into tinhub by deploy/mirror-sync.sh, with a mirror account's key; twice, the second run
# carrying only a new commit
"$driver" repo mirror
mkdir -p "$tmp/bot"
botk() {
	HOME="$tmp/bot" XDG_CONFIG_HOME="$tmp/bot" "$tit" "$@"
}
botk config set --user user.name Mirror
botk config set --user user.email mirror@example.com
"$hub" admin invite mirror@example.com > "$tmp/invite.out" || fail "the mirror's invite"
code=$(sed -n 's/^invite for mirror@example.com: \(.*\)$/\1/p' "$tmp/invite.out")
botk key add "$base" "$code" > "$tmp/out" 2>&1 || fail "the mirror's key: $(cat "$tmp/out")"
"$driver" grant mirror@example.com write mirror
export GIT_AUTHOR_NAME=Ada GIT_AUTHOR_EMAIL=ada@example.com GIT_COMMITTER_NAME=Ada GIT_COMMITTER_EMAIL=ada@example.com
git init -q -b main "$tmp/src"
printf 'one\n' > "$tmp/src/a.txt"
git -C "$tmp/src" add a.txt
git -C "$tmp/src" commit -q -m first
git clone -q "$tmp/src" "$tmp/mirror"
(cd "$tmp/mirror" && botk adopt > /dev/null 2>&1 && botk remote add tinhub "$base/ada/mirror") || fail "adopting the git clone"
HOME="$tmp/bot" XDG_CONFIG_HOME="$tmp/bot" TIT="$tit" sh "$deploy/mirror-sync.sh" "$tmp/mirror" > "$tmp/out" 2>&1 || fail "mirror-sync: $(cat "$tmp/out")"
printf 'two\n' > "$tmp/src/b.txt"
git -C "$tmp/src" add b.txt
git -C "$tmp/src" commit -q -m second
HOME="$tmp/bot" XDG_CONFIG_HOME="$tmp/bot" TIT="$tit" sh "$deploy/mirror-sync.sh" "$tmp/mirror" > "$tmp/out" 2>&1 || fail "mirror-sync again: $(cat "$tmp/out")"
(cd "$tmp" && bob clone "$base/ada/mirror" copy > "$tmp/out" 2>&1) || fail "cloning the mirror: $(cat "$tmp/out")"
[ "$(cat "$tmp/copy/a.txt")" = one ] && [ "$(cat "$tmp/copy/b.txt")" = two ] || fail "the mirror's files: $(ls "$tmp/copy")"
echo "PASS tinhub mirror: a git repository synced twice, then cloned"

# restarts under load lose no push: bob pushes 20 commits one after another, retrying a push the restart cut off,
# while the server is stopped (SIGTERM, a drain) and started again three times; every file is on main after
cd "$tmp/work"
bob pull > /dev/null 2>&1 || true
(
	for i in $(seq 1 20); do
		printf '%s\n' "$i" > "load$i.txt"
		bob add "load$i.txt"
		bob commit -m "load $i" > /dev/null
		n=0
		until bob push > "$tmp/push.out" 2>&1; do
			n=$((n + 1))
			[ $n -lt 100 ] || { echo "push $i: $(cat "$tmp/push.out")"; exit 1; }
			sleep 0.1
		done
	done
) > "$tmp/load.out" 2>&1 &
load=$!
for r in 1 2 3; do
	sleep 0.5
	kill -TERM "$pid"
	wait "$pid" || true
	start
done
wait $load || fail "the pushes under restarts: $(cat "$tmp/load.out")"
cd "$tmp"
rm -rf copy2
bob clone "$base/ada/tin" copy2 > "$tmp/out" 2>&1 || fail "the clone after the restarts: $(cat "$tmp/out")"
for i in $(seq 1 20); do
	[ "$(cat "$tmp/copy2/load$i.txt" 2>/dev/null)" = "$i" ] || fail "push $i was lost across the restarts"
done
echo "PASS tinhub restart: 20 pushes across three restarts, none lost"
