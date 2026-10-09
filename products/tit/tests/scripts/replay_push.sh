#!/bin/sh
# tit replay (#1027, design/tit.md §17): a refused request tit serve records as a version 2 capsule (sealed for a reader,
# signed by the server) is sent to the repository's host with tit replay push, which empties the spool and sends a
# capsule again as no new one; a version 1 capsule is refused; tit replay ls lists it and its group; an unsigned client
# lists nothing; tit replay ID fetches it, and with --against runs tin replay on it with the reader's key: the same
# status, no divergence. Usage: replay_push.sh <tit> <dir>
set -eu
tit=$1
d=$2
root=$PWD
export HOME="$d/home" XDG_CONFIG_HOME="$d/home" TIT_NO_PAGER=1
fail() {
	echo "FAIL replay_push: $*"
	exit 1
}
t() { "$tit" "$@"; }
mkdir -p "$HOME" "$d/stranger" "$d/spool"
t config set --user user.name Ada
t config set --user user.email ada@example.com
pub=$(t key 2>/dev/null)
reader=$(cd "$root" && sh ./tin replay key "$d/reader.key")
seed=$(openssl rand -hex 32)
mkdir "$d/server"
cd "$d/server"
t init . > /dev/null
printf 'one\n' > a.txt
t add a.txt
t commit -m first > /dev/null
echo "ada@example.com ed25519 $pub" > .tit/allowed-keys
port=$((20000 + ($$ + 3313) % 20000))
url="http://127.0.0.1:$port/"
TIN_REPLAY_DIR="$d/spool" TIN_REPLAY_KEY=$(openssl rand -hex 32) TIN_REPLAY_RECIPIENTS=$reader TIN_REPLAY_SIGNING_KEY=$seed TIN_REPLAY_SAMPLE=1 TIN_CORES=1 "$tit" serve --addr "127.0.0.1:$port" > "$d/serve.log" 2>&1 &
server=$!
trap 'kill $server 2> /dev/null || true' EXIT HUP INT TERM
cd "$d"
n=0
until t clone "$url" c > "$d/clone.txt" 2>&1; do
	n=$((n + 1))
	[ $n -lt 50 ] || fail "the server did not start: $(cat "$d/serve.log")"
	rm -rf c
	sleep 0.1
done
cd "$d/c"
# the failure to record: a stranger's clone of the private repository ("$tit", not t: an assignment before a shell
# function stays set after it); a push would not be recorded, as anvil records no streamed request
if HOME="$d/stranger" XDG_CONFIG_HOME="$d/stranger" "$tit" clone "$url" "$d/stranger-c" > "$d/push.txt" 2>&1; then
	fail "an unsigned clone of a private repository went through"
fi
sleep 0.3
capsule=$(ls "$d/spool" | grep '\.tcap$' | sort | tail -1)
[ -n "$capsule" ] || fail "no capsule for the refused clone"
cp "$d/spool/$capsule" "$d/again.tcap"
# the same server, recording off: with every request sampled it would record the capsules' own pushes
kill $server
wait $server 2> /dev/null || true
(cd "$d/server" && exec "$tit" serve --addr "127.0.0.1:$port") > "$d/serve.log" 2>&1 &
server=$!
n=0
until t replay ls > /dev/null 2>&1; do
	n=$((n + 1))
	[ $n -lt 50 ] || fail "the server did not start again: $(cat "$d/serve.log")"
	sleep 0.1
done

t replay push "$d/spool" > "$d/out.txt" 2>&1 || fail "replay push: $(cat "$d/out.txt")"
grep -q "^Sent [1-9][0-9]* capsules to origin ([1-9][0-9]* new)\.$" "$d/out.txt" || fail "replay push said: $(cat "$d/out.txt")"
[ -z "$(ls "$d/spool" | grep '\.tcap$' || true)" ] || fail "the spool still holds capsules"
mkdir "$d/spool2"
cp "$d/again.tcap" "$d/spool2/"
t replay push "$d/spool2" > "$d/out.txt" 2>&1 || fail "replay push again: $(cat "$d/out.txt")"
grep -q "(0 new)" "$d/out.txt" || fail "a capsule sent again was new: $(cat "$d/out.txt")"
echo "ok replay push sends the spool once and again as nothing new"

# a version 1 capsule is refused: the server could not group it
mkdir "$d/spool1"
printf 'TINCAP\001\000not really' > "$d/spool1/old.tcap"
if t replay push "$d/spool1" > "$d/out.txt" 2>&1; then
	fail "a version 1 capsule was taken"
fi
grep -q "version 2" "$d/out.txt" || fail "the refusal: $(cat "$d/out.txt")"
echo "ok a version 1 capsule is refused"

t replay ls > "$d/ls.txt"
line=$(grep "401 GET /tit/v1/heads" "$d/ls.txt" | head -1)
[ -n "$line" ] || fail "replay ls: $(cat "$d/ls.txt")"
id=$(echo "$line" | awk '{print $1}')
t replay ls --group > "$d/groups.txt"
grep -q "401 GET /tit/v1/heads" "$d/groups.txt" || fail "replay ls --group: $(cat "$d/groups.txt")"
if HOME="$d/stranger" XDG_CONFIG_HOME="$d/stranger" "$tit" replay ls > "$d/out.txt" 2>&1; then
	fail "an unsigned client listed capsules"
fi
echo "ok replay ls lists the capsule and its group, and not to a stranger"

t replay "$id" > "$d/got.txt" || fail "replay ID: $(cat "$d/got.txt")"
signer=$(sed -n 's/.*signed by \([0-9a-f]*\)$/\1/p' "$d/got.txt")
[ ${#signer} -eq 64 ] || fail "the signer: $(cat "$d/got.txt")"
printf '#!/bin/sh\ncd "%s/server" && exec "%s" serve --addr 127.0.0.1:1\n' "$d" "$tit" > "$d/serve.sh"
chmod +x "$d/serve.sh"
status=0
TIT_TIN="$root/tin" TIN_REPLAY_IDENTITY="$d/reader.key" TIN_REPLAY_SIGNERS=$signer t replay "$id" --against "$d/serve.sh" > "$d/replay.txt" 2>&1 || status=$?
[ $status = 0 ] || fail "tit replay --against exited $status: $(cat "$d/replay.txt")"
grep -aq "^replay: status 401 (recorded 401)" "$d/replay.txt" || fail "the replay: $(cat "$d/replay.txt")"
grep -aq "divergence" "$d/replay.txt" && fail "the replay diverged: $(cat "$d/replay.txt")"
echo "ok tit replay ID --against replays the capsule with the reader's key"
