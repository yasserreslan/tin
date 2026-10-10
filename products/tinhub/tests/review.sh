#!/bin/sh
# Reviews end to end (#1025) against TINHUB_TEST_DB. Usage: review.sh REVIEW_NODE TIT. A node (tests/programs/review_node:
# the protocol, the review routes, and a worker running the push fan-out and the review jobs) takes pushes over tit's
# protocol, signed with bob's key, that carry change versions: a stack of three changes on main. Then, through the API:
#   - the fan-out opened a review for each change; carol (a writer) approves them;
#   - the stack cannot land without the required check, which ci posts signed with its key (401 and a nonce first);
#   - main moves, and landing the stack from its top rebases each change on the server and moves main once per change,
#     bottom first; tit clones the result;
#   - a change that does not rebase onto the new main is refused, and main stays where it is.
set -eu
node=$1
tit=$2
tmp=$(mktemp -d)
pid=
cleanup() {
	[ -n "$pid" ] && kill "$pid" 2>/dev/null || true
	rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
fail() {
	echo "FAIL tinhub review: $*"
	[ -f "$tmp/node.log" ] && tail -n 20 "$tmp/node.log"
	exit 1
}

addr=127.0.0.1:18461
base=http://$addr
"$node" setup "$tmp/node" > /dev/null || fail "setup"
"$node" serve "$addr" "$tmp/node" > "$tmp/node.log" 2>&1 &
pid=$!
up=0
for i in $(seq 1 40); do
	curl -s -o /dev/null "$base/api/v1/repos/ada/tin/reviews" 2>/dev/null && { up=1; break; }
	sleep 0.25
done
[ $up = 1 ] || fail "the node did not start"

# call USER METHOD PATH [BODY]: the status, with the answer in $tmp/out.json
call() {
	if [ $# -ge 4 ]; then
		curl -s -o "$tmp/out.json" -w '%{http_code}' -X "$2" -H "X-Test-User: $1" -H 'Content-Type: application/json' --data "$4" "$base$3"
	else
		curl -s -o "$tmp/out.json" -w '%{http_code}' -X "$2" -H "X-Test-User: $1" "$base$3"
	fi
}
expect() {
	[ "$1" = "$2" ] || fail "$3: status $1, not $2: $(cat "$tmp/out.json")"
}
# wait_for PATH JQ: poll GET PATH (as carol) until JQ holds
wait_for() {
	for i in $(seq 1 80); do
		code=$(call 3 GET "$1")
		[ "$code" = 200 ] && jq -e "$2" "$tmp/out.json" > /dev/null 2>&1 && return 0
		sleep 0.25
	done
	fail "GET $1 never held $2: $(cat "$tmp/out.json")"
}

"$node" ids > "$tmp/ids"
id() { sed -n "s/^$1 \([^ ]*\) .*/\1/p" "$tmp/ids"; }
commit() { sed -n "s/^$1 [^ ]* \(.*\)/\1/p" "$tmp/ids"; }
A=$(id A)
B=$(id B)
C=$(id C)
D=$(id D)
r=/api/v1/repos/ada/tin

"$node" push "$base" "$tmp/node" base > /dev/null || fail "the push of main"
"$node" push "$base" "$tmp/node" stack > /dev/null || fail "the push of the stack"
for ch in $A $B $C; do
	wait_for "$r/changes/$ch/review" '.state == "open" and .version == 1 and .target == "refs/heads/main" and .opened_by == 2'
done
echo "PASS tinhub review: a push carrying three change versions opens three reviews"

for ch in $A $B $C; do
	expect "$(call 3 POST "$r/changes/$ch/approvals" '{"vote":"approve"}')" 200 "carol approves $ch"
	jq -e '.state == "approved"' "$tmp/out.json" > /dev/null || fail "approved: $(cat "$tmp/out.json")"
done
expect "$(call 3 GET "$r/changes/$C/review")" 200 "C's review"
jq -e '.below == ["'"$A"'", "'"$B"'"] and (.blocked | contains("ci")) and (.stack_blocked | contains("ci"))' "$tmp/out.json" > /dev/null || fail "C without the check: $(cat "$tmp/out.json")"
expect "$(call 3 POST "$r/changes/$C/land" '{"stack":true}')" 409 "the stack without its check"
jq -e '.code == "not_ready" and (.message | contains("ci none")) and (.landed | length) == 0' "$tmp/out.json" > /dev/null || fail "without the check: $(cat "$tmp/out.json")"
for ch in $A $B $C; do
	"$node" check "$base" "$tmp/node" "$ch" success > "$tmp/check.out" 2>&1 || fail "ci's signed check on $ch: $(cat "$tmp/check.out")"
done
wait_for "$r/changes/$A/checks" '(.checks | length) == 1 and .checks[0].state == "success" and .checks[0].key_id > 0'
wait_for "$r/changes/$C/review" '.below == ["'"$A"'", "'"$B"'"] and .stack_blocked == "" and (.blocked | contains("'"$B"' below it has not landed"))'
expect "$(call 3 POST "$r/changes/$C/land" '{"stack":false}')" 409 "C alone, above B"
echo "PASS tinhub review: a change cannot land without its required check; ci posts it signed with its key"
echo "PASS tinhub review: the top of a stack is ready only with the stack, and lands alone only once the changes below have"

"$node" push "$base" "$tmp/node" move > /dev/null || fail "the push that moves main"
m1=$(commit m1)
expect "$(call 3 POST "$r/changes/$C/land" '{"stack":true}')" 200 "the stack lands"
jq -e '(.landed | length) == 3 and .landed[0].change == "'"$A"'" and .landed[1].change == "'"$B"'" and .landed[2].change == "'"$C"'" and all(.landed[]; .rebased and .version == 2)' "$tmp/out.json" > /dev/null || fail "what landed: $(cat "$tmp/out.json")"
a2=$(jq -r '.landed[0].commit' "$tmp/out.json")
b2=$(jq -r '.landed[1].commit' "$tmp/out.json")
c2=$(jq -r '.landed[2].commit' "$tmp/out.json")
"$node" log "$base" "$tmp/node" refs/heads/main > "$tmp/log"
[ "$(cut -d' ' -f1 "$tmp/log" | head -n 4 | tr '\n' ' ')" = "$c2 $b2 $a2 $m1 " ] || fail "main is not C, B, A on the moved main: $(cat "$tmp/log")"
[ "$(sed -n 2p "$tmp/log" | cut -d' ' -f2)" = "$B" ] || fail "the rebased B keeps its change id: $(cat "$tmp/log")"
for ch in $A $B $C; do
	wait_for "$r/changes/$ch/review" '.state == "landed" and .landed_by == 3 and .version == 2 and .blocked == "the review is landed"'
done
wait_for "$r/reviews?state=landed" '(.reviews | length) == 3'
echo "PASS tinhub review: the stack lands bottom-up, rebased onto the moved main"

# one move of main per change, in order: the push events after the base and the move
"$node" moves > "$tmp/moves"
m0=$(commit m0)
printf ' %s\n%s %s\n%s %s\n%s %s\n%s %s\n' "$m0" "$m0" "$m1" "$m1" "$a2" "$a2" "$b2" "$b2" "$c2" > "$tmp/moves.want"
cmp -s "$tmp/moves" "$tmp/moves.want" || fail "main's moves: $(cat "$tmp/moves"), not $(cat "$tmp/moves.want")"
echo "PASS tinhub review: main moved once per change, in order"

export TIT_NO_PAGER=1
(cd "$tmp" && HOME="$tmp" XDG_CONFIG_HOME="$tmp" "$tit" clone "$base/ada/tin" clone > clone.out 2>&1) || fail "tit clone: $(cat "$tmp/clone.out")"
[ "$(cat "$tmp/clone/c.tin" | sed -n 4p)" = "	return 2" ] || fail "the clone's c.tin: $(cat "$tmp/clone/c.tin")"
[ "$(cat "$tmp/clone/README")" = "tin, the language" ] || fail "the clone's README"
echo "PASS tinhub review: tit clones the landed main"

# a change made on the old main that does not rebase onto the moved one is refused, and main stays
"$node" push "$base" "$tmp/node" conflict > /dev/null || fail "the push of D"
wait_for "$r/changes/$D/review" '.state == "open"'
expect "$(call 3 POST "$r/changes/$D/approvals" '{"vote":"approve"}')" 200 "carol approves D"
"$node" check "$base" "$tmp/node" "$D" success > /dev/null 2>&1 || fail "ci's check on D"
expect "$(call 3 POST "$r/changes/$D/land")" 409 "D does not rebase"
jq -e '.code == "conflict" and (.message | contains("README"))' "$tmp/out.json" > /dev/null || fail "the refusal: $(cat "$tmp/out.json")"
"$node" log "$base" "$tmp/node" refs/heads/main > "$tmp/log2"
[ "$(head -n 1 "$tmp/log2" | cut -d' ' -f1)" = "$c2" ] || fail "main moved after a refused landing"
wait_for "$r/changes/$D/review" '.state == "approved" and .version == 1'
echo "PASS tinhub review: a landing whose rebase conflicts is refused"
