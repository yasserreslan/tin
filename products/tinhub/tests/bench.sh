#!/bin/sh
# Benchmark history and releases (#1029) against TINHUB_TEST_DB. Usage: bench.sh TINHUB REPO_DRIVER TIT BENCH_NODE
# COMPILER. A tinhub node takes tit's pushes and BENCH_NODE (programs/bench_node.tin: the bench routes and a worker with
# the workers' and bench's handlers) runs the jobs. The Tin repo's bench/fib.tin is built with COMPILER and timed here
# (the median of three runs), recorded with tit bench record and committed under .bench/; the push is read into
# bench_results with the machine's CPU and kernel, and a macOS line beside it is stored but charted on its own. A change
# that makes fib deliberately slower (it computes fib three times) gets a failed bench check and a note on its review;
# tit ship's release is listed with its changelog and a verified signature.
set -eu
hub=$1
driver=$2
tit=$3
node=$4
compiler=$5
root=$(cd "$(dirname "$0")/../../.." && pwd)
case $compiler in /*) ;; *) compiler=$root/$compiler ;; esac
tmp=$(mktemp -d)
pids=
cleanup() {
	for p in $pids; do kill "$p" 2>/dev/null || true; done
	rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
fail() {
	echo "FAIL tinhub bench: $*"
	for f in hub.log node.log out; do
		[ -f "$tmp/$f" ] && { echo "--- $f"; tail -n 20 "$tmp/$f"; }
	done
	exit 1
}

"$driver" setup "$tmp/packs" > /dev/null
"$driver" repo fib
dbaddr=${TINHUB_TEST_DB%%/*}
export TINHUB_DB_ADDR="$dbaddr" TINHUB_DB_NAME="${TINHUB_TEST_DB#*/}" TINHUB_DB_USER="${TINHUB_TEST_DB_USER:-}" TINHUB_DB_PASSWORD="${TINHUB_TEST_DB_PASSWORD:-}"
export TINHUB_SECRETS_NONCE=test-nonce-secret-0123456789 TINHUB_SECRETS_COOKIE=test-cookie-key-0123456789
# the directory store: the driver writes the repository to disk (production uses a bucket: backup.sh, phase2.sh)
export TINHUB_PACKS_STORE=dir TINHUB_PACKS_SWEEP=0 TINHUB_PACKS_DIR="$tmp/packs" TIT_NO_PAGER=1
hubaddr=127.0.0.1:18461
nodeaddr=127.0.0.1:18462
TINHUB_LISTEN=$hubaddr TINHUB_PUBLIC_URL=http://$hubaddr TINHUB_ROLES=node "$hub" run > "$tmp/hub.log" 2>&1 &
pids="$pids $!"
"$node" serve "$nodeaddr" "$tmp/packs" > "$tmp/node.log" 2>&1 &
pids="$pids $!"
api=http://$nodeaddr/api/v1/repos/ada/fib
for url in "http://$hubaddr/healthz" "$api/bench"; do
	up=0
	for i in $(seq 1 40); do
		curl -sf "$url" > /dev/null 2>&1 && { up=1; break; }
		sleep 0.25
	done
	[ $up = 1 ] || fail "$url did not answer"
done

# get PATH JQ: GET PATH (under the repository's API) is 200 JSON and JQ holds on it
get() {
	code=$(curl -s -o "$tmp/out.json" -w '%{http_code}' "$api$1")
	[ "$code" = 200 ] || fail "GET $1: $code $(cat "$tmp/out.json")"
	jq -e "$2" "$tmp/out.json" > /dev/null || fail "GET $1: $(cat "$tmp/out.json") does not hold $2"
}
# until PATH JQ: GET PATH until JQ holds, for 20 s (the worker's jobs run after the push answers)
until_() {
	for i in $(seq 1 80); do
		curl -sf -o "$tmp/out.json" "$api$1" 2> /dev/null && jq -e "$2" "$tmp/out.json" > /dev/null 2>&1 && return 0
		sleep 0.25
	done
	fail "GET $1 never held $2: $(cat "$tmp/out.json" 2> /dev/null)"
}
kit() {
	HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/home" "$tit" "$@"
}
# timed FILE: build it and print the median of three runs, in ms
timed() {
	# (a subshell: the failure goes to stderr)
	TIN_ROOT=$root "$compiler" -o "$tmp/prog" "$1" > "$tmp/build.out" 2>&1 || { cat "$tmp/build.out" >&2; echo "FAIL tinhub bench: building $1" >&2; exit 1; }
	for i in 1 2 3; do
		t0=$(date +%s%N)
		"$tmp/prog" > /dev/null
		echo $((($(date +%s%N) - t0) / 1000000))
	done | sort -n | sed -n 2p
}
# record MS: tit bench record for HEAD's change, then the results committed under .bench/ (amending the same change)
record() {
	kit bench record fib "$1" --unit ms > "$tmp/out" 2>&1 || fail "tit bench record: $(cat "$tmp/out")"
	mkdir -p .bench
	cp .tit/bench/fib.jsonl .bench/fib.jsonl
	kit add .bench/fib.jsonl
	kit commit --amend -m "$2" > "$tmp/out" 2>&1 || fail "tit commit --amend: $(cat "$tmp/out")"
}

mkdir -p "$tmp/home"
kit config set --user user.name Bench > /dev/null
kit config set --user user.email bench@example.com > /dev/null
"$hub" admin invite bench@example.com > "$tmp/invite.out" || fail "the invite"
code=$(sed -n 's/^invite for bench@example.com: \(.*\)$/\1/p' "$tmp/invite.out")
kit key add "http://$hubaddr" "$code" > "$tmp/out" 2>&1 || fail "tit key add: $(cat "$tmp/out")"
"$driver" grant bench@example.com write fib

# main: the Tin repo's fib benchmark, timed and recorded
kit init "$tmp/work" > /dev/null
cd "$tmp/work"
kit remote add origin "http://$hubaddr/ada/fib" > /dev/null
cp "$root/bench/fib.tin" fib.tin
kit add fib.tin
kit commit -m "fib" > /dev/null
base=$(timed fib.tin)
record "$base" "fib"
# a macOS result of the same change (a development number): stored, never compared with Linux, charted on its own
sed -n '1s/"Os":"Linux"/"Os":"Darwin"/p' .bench/fib.jsonl | sed 's/"Value":"[0-9.]*"/"Value":"1"/' >> .bench/fib.jsonl
grep -q '"Os":"Darwin"' .bench/fib.jsonl || fail "the macOS line"
kit add .bench/fib.jsonl
kit commit --amend -m "fib" > /dev/null
kit push > "$tmp/out" 2>&1 || fail "tit push main: $(cat "$tmp/out")"
until_ /bench '.benchmarks[0].name == "fib" and .benchmarks[0].linux_results == 1 and .benchmarks[0].results == 2'
cpu=$(sed -n 's/^model name[[:space:]]*:[[:space:]]*//p' /proc/cpuinfo | head -n 1)
get /bench/fib '(.series | length) == 2 and .series[0].os == "Linux" and .series[0].development == false and .series[0].points[0].kernel == "'"$(uname -r)"'" and .series[1].os == "Darwin" and .series[1].development == true'
[ -z "$cpu" ] || jq -e --arg cpu "$cpu" '.series[0].cpu == $cpu' "$tmp/out.json" > /dev/null || fail "the CPU is not recorded: $(cat "$tmp/out.json")"
echo "PASS tinhub bench: the results pushed are read, with the CPU and kernel; macOS apart (fib $base ms)"

# a change made deliberately slower: fib three times
kit switch -c slow > /dev/null
sed 's/say.Line(fib(35))/say.Line(fib(35) + fib(35) - fib(35))/' "$root/bench/fib.tin" > fib.tin
grep -q 'fib(35) + fib(35)' fib.tin || fail "the slower fib"
kit add fib.tin
kit commit -m "fib, three times" > /dev/null
slow=$(timed fib.tin)
record "$slow" "fib, three times"
change=$(kit log -n 1 --format=change)
commit=$(kit log -n 1 --format=id)
# what reviews (#1025) record when the change is pushed for review: its version and its open review
"$node" version ada fib "$change" "$commit"
kit push > "$tmp/out" 2>&1 || fail "tit push slow: $(cat "$tmp/out")"
for i in $(seq 1 80); do
	"$node" checks > "$tmp/checks"
	grep -q "^$change 1 " "$tmp/checks" && break
	sleep 0.25
done
grep -q "^$change 1 failure$" "$tmp/checks" || fail "the slower change's check: $(cat "$tmp/checks") (fib $base ms, then $slow ms)"
"$node" notes > "$tmp/notes"
grep -q '^- fib: .* ms, .*% slower (Linux ' "$tmp/notes" || fail "the note: $(cat "$tmp/notes")"
# a first version's diff is against what it applies to, main: two files modified, not the whole tree added
curl -sf -o "$tmp/diff.json" "http://$hubaddr/api/v1/repos/ada/fib/changes/$change/diff" || fail "GET the change's diff"
jq -e '.from == 0 and .to == 1 and ([.files[].path] | sort) == [".bench/fib.jsonl", "fib.tin"] and all(.files[]; .kind == "modified")' "$tmp/diff.json" > /dev/null || fail "the change's diff: $(cat "$tmp/diff.json")"
get "/changes/$change/bench" '.state == "failure" and .version == 1 and .benchmarks[0].name == "fib" and .benchmarks[0].regressed == true and .benchmarks[0].slower > 0.1'
# the function reviews call when a review opens: the same verdict, and still one note
"$node" compare ada fib "$change" 1 "$tmp/packs" > "$tmp/out" || fail "compare"
grep -q '^state failure$' "$tmp/out" || fail "compare: $(cat "$tmp/out")"
"$node" notes > "$tmp/notes2"
[ "$(grep -c '^Benchmarks slower' "$tmp/notes2")" = 1 ] || fail "a second note: $(cat "$tmp/notes2")"
echo "PASS tinhub bench: a change made slower ($base ms, then $slow ms) fails its check, with a note on its review"

# a release: tit ship tags main with the changelog, signed with the key tinhub knows
kit switch main > /dev/null
kit ship v1.0 > "$tmp/out" 2>&1 || fail "tit ship: $(cat "$tmp/out")"
get /releases '(.releases | length) == 1 and .releases[0].name == "v1.0" and .releases[0].title == "v1.0" and .releases[0].signed == true and .releases[0].verified == true and .releases[0].signed_by == "bench" and (.releases[0].changes | length) == 1 and .releases[0].changes[0].title == "fib"'
get /releases/v1.0 '.name == "v1.0" and (.commit | length) == 64 and .tagger.email == "bench@example.com"'
code=$(curl -s -o /dev/null -w '%{http_code}' "$api/releases/v2.0")
[ "$code" = 404 ] || fail "a missing release: $code"
echo "PASS tinhub bench: tit ship's release, with its changelog and a verified signature"
