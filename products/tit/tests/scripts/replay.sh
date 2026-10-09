#!/bin/sh
# A refused request recorded by tit serve replays with tin replay (#796): the server runs with anvil's recording on
# (TIN_REPLAY_DIR, TIN_REPLAY_KEY, every request sampled); an unsigned clone of the private repository is refused and
# leaves a capsule, and tin replay runs it again against tit serve with nothing listening: the same status, no
# divergence, every recorded effect used. (A push is not the request recorded: pushes and fetches are read as they
# arrive, on anvil's Router.Stream routes, and anvil records no streamed request.) Usage: replay.sh <tit> <dir>
set -eu
tit=$1
d=$2
root=$PWD
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1
fail() {
	echo "FAIL replay: $*"
	exit 1
}
t() { "$tit" "$@"; }
mkdir -p "$HOME" "$d/stranger" "$d/spool"
t config set --user user.name Ada
t config set --user user.email ada@example.com
pub=$(t key 2>/dev/null)
mkdir "$d/server"
cd "$d/server"
t init . > /dev/null
printf 'one\n' > a.txt
t add a.txt
t commit -m first > /dev/null
echo "ada@example.com ed25519 $pub" > .tit/allowed-keys
key=$(openssl rand -hex 32)
port=$((20000 + $$ % 20000))
TIN_REPLAY_DIR="$d/spool" TIN_REPLAY_KEY=$key TIN_REPLAY_SAMPLE=1 TIN_CORES=1 "$tit" serve --addr "127.0.0.1:$port" > "$d/serve.log" 2>&1 &
server=$!
trap 'kill $server 2> /dev/null || true' EXIT HUP INT TERM
cd "$d"
n=0
until t clone "http://127.0.0.1:$port/" c > "$d/clone.txt" 2>&1; do
	n=$((n + 1))
	[ $n -lt 50 ] || fail "the server did not start: $(cat "$d/serve.log")"
	rm -rf c
	sleep 0.1
done
before=$(ls "$d/spool" | wc -l)
# "$tit", not t: an assignment before a shell function stays set after it
if HOME="$d/stranger" XDG_CONFIG_HOME="$d/stranger" "$tit" clone "http://127.0.0.1:$port/" "$d/stranger-c" > "$d/push.txt" 2>&1; then
	fail "an unsigned clone of a private repository went through"
fi
grep -q Unauthorized "$d/push.txt" || fail "the clone: $(cat "$d/push.txt")"
sleep 0.2
kill $server
wait $server 2> /dev/null || true
capsule=$(ls "$d/spool" | sort | tail -1)
[ "$(ls "$d/spool" | wc -l)" -gt "$before" ] || fail "no capsule for the refused clone"
printf '#!/bin/sh\ncd "%s/server" && exec "%s" serve --addr 127.0.0.1:1\n' "$d" "$tit" > "$d/serve.sh"
chmod +x "$d/serve.sh"
status=0
(cd "$root" && TIN_REPLAY_KEY=$key sh ./tin replay "$d/spool/$capsule" --against "$d/serve.sh") > "$d/replay.txt" 2>&1 || status=$?
[ $status = 0 ] || fail "tin replay exited $status: $(cat "$d/replay.txt")"
grep -aq "^replay: status 401 (recorded 401)" "$d/replay.txt" || fail "the replay: $(cat "$d/replay.txt")"
grep -aq "divergence" "$d/replay.txt" && fail "the replay diverged: $(cat "$d/replay.txt")"
echo "ok a refused request recorded by tit serve replays with tin replay"
