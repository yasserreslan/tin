#!/bin/sh
# tit login, tit invite and tit key add (#1017, design/tit.md §18), against tit serve: a login request a browser makes
# (unsigned, on a private repository) is approved once by a registered key and then reads as approved; a second
# approval, an unknown key and an expired request are refused; an invite lets a new key in once, and only for the
# email it names. Usage: login.sh <tit> <dir>
set -eu
tit=$1
d=$2
export HOME="$d/ada" XDG_CONFIG_HOME="$d/ada" TIT_NO_PAGER=1
fail() {
	echo "FAIL login: $*"
	exit 1
}
t() { "$tit" "$@"; }
# as() runs tit with another person's home: "$tit" with its own assignments, not t (they would stay set after it)
as() {
	who=$1
	shift
	HOME="$d/$who" XDG_CONFIG_HOME="$d/$who" "$tit" "$@"
}
mkdir -p "$d/ada" "$d/bob" "$d/eve"
t config set --user user.name Ada
t config set --user user.email ada@example.com
pub=$(t key 2>/dev/null)
for p in bob eve; do
	as $p config set --user user.name "$p"
	as $p config set --user user.email "$p@example.com"
	as $p key > /dev/null 2>&1
done
mkdir "$d/server"
cd "$d/server"
t init . > /dev/null
printf 'one\n' > a.txt
t add a.txt
t commit -m first > /dev/null
echo "ada@example.com ed25519 $pub" > .tit/allowed-keys
port=$((20000 + ($$ + 2711) % 20000))
url="http://127.0.0.1:$port"
TIT_SERVE_LOGIN_TTL=2 TIN_CORES=2 "$tit" serve --addr "127.0.0.1:$port" > "$d/serve.log" 2>&1 &
server=$!
trap 'kill $server 2> /dev/null || true' EXIT HUP INT TERM
cd "$d"
n=0
until t clone "$url/" c > /dev/null 2>&1; do
	n=$((n + 1))
	[ $n -lt 50 ] || fail "the server did not start: $(cat "$d/serve.log")"
	rm -rf c
	sleep 0.1
done

# a browser asks for a login request, unsigned; Ada approves it; it reads as approved; approving it again is refused
curl -s -X POST "$url/tit/v1/login" > "$d/req.json"
code=$(sed -n 's/.*"code":"\([A-Z0-9-]*\)".*/\1/p' "$d/req.json")
[ ${#code} -eq 9 ] || fail "the login request: $(cat "$d/req.json")"
curl -s "$url/tit/v1/login/$code" | grep -q '"state":"pending"' || fail "a new request is not pending"
(cd c && t login --code "$code") > "$d/out.txt" 2>&1 || fail "tit login --code: $(cat "$d/out.txt")"
grep -q "^Login request $code approved as ada@example.com\.$" "$d/out.txt" || fail "tit login said: $(cat "$d/out.txt")"
curl -s "$url/tit/v1/login/$code" > "$d/state.json"
grep -q '"state":"approved"' "$d/state.json" && grep -q '"email":"ada@example.com"' "$d/state.json" || fail "the state: $(cat "$d/state.json")"
if (cd c && t login --code "$code") > "$d/out.txt" 2>&1; then
	fail "a code was approved twice"
fi
grep -q "approved already" "$d/out.txt" || fail "the second approval: $(cat "$d/out.txt")"
echo "ok a login request is approved once with a key"

# tit login on its own makes and approves one
(cd c && t login) > "$d/out.txt" 2>&1 || fail "tit login: $(cat "$d/out.txt")"
grep -q "approved as ada@example.com" "$d/out.txt" && grep -q "A browser waiting on $url/tit/v1/login/" "$d/out.txt" || fail "tit login said: $(cat "$d/out.txt")"
echo "ok tit login makes a request and approves it"

# an unknown key, and an expired request, are refused
curl -s -X POST "$url/tit/v1/login" > "$d/req2.json"
code2=$(sed -n 's/.*"code":"\([A-Z0-9-]*\)".*/\1/p' "$d/req2.json")
if (cd c && as eve login --code "$code2") > "$d/out.txt" 2>&1; then
	fail "an unknown key approved a login"
fi
grep -q "Unauthorized" "$d/out.txt" || fail "the unknown key's refusal: $(cat "$d/out.txt")"
sleep 3
curl -s "$url/tit/v1/login/$code2" | grep -q '"state":"expired"' || fail "an old request is not expired"
if (cd c && t login --code "$code2") > "$d/out.txt" 2>&1; then
	fail "an expired request was approved"
fi
grep -q "expired" "$d/out.txt" || fail "the expired refusal: $(cat "$d/out.txt")"
echo "ok an unknown key and an expired request are refused"

# an invite for bob lets bob's key in, once; eve cannot use it; bob then reads the private repository
inv=$(cd "$d/server" && t invite --email bob@example.com | awk '{print $2}')
[ ${#inv} -eq 9 ] || fail "the invite: $inv"
if as eve key add "$url" "$inv" > "$d/out.txt" 2>&1; then
	fail "eve used bob's invite"
fi
grep -q "is for bob@example.com" "$d/out.txt" || fail "eve's refusal: $(cat "$d/out.txt")"
as bob key add "$url" "$inv" > "$d/out.txt" 2>&1 || fail "bob's key add: $(cat "$d/out.txt")"
grep -q "^Added the key of bob@example.com at $url\.$" "$d/out.txt" || fail "key add said: $(cat "$d/out.txt")"
as bob clone "$url/" "$d/bobs" > "$d/out.txt" 2>&1 || fail "bob's clone after his key was added: $(cat "$d/out.txt")"
if as bob key add "$url" "$inv" > "$d/out.txt" 2>&1; then
	fail "an invite was used twice"
fi
grep -q "spent" "$d/out.txt" || fail "the spent invite: $(cat "$d/out.txt")"
grep -q "^bob@example.com ed25519 " "$d/server/.tit/allowed-keys" || fail "bob's key is not in the allowed keys"
echo "ok an invite lets one key in, for its email, once"
