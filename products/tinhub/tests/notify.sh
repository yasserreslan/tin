#!/bin/sh
# Notifications, webhooks and live updates (#1023) against TINHUB_TEST_DB. Usage: notify.sh NOTIFY_NODE HOOKS, where
# HOOKS is bench/ref/tinhub_hooks built (a webhook receiver that checks signatures with Go's crypto/hmac, and an SMTP
# sink). Two nodes run: A (with the worker) and B. A websocket on B sees an event committed through A; webhooks are
# created through the API by the repository's admin, a correctly signed delivery is accepted, a 500 is tried again and
# a delivery that keeps failing is given up after 3 attempts, each attempt in deliveries; a follower is mailed.
set -eu
node=$1
hooks=$2
tmp=$(mktemp -d)
pids=
cleanup() {
	for p in $pids; do kill "$p" 2>/dev/null || true; done
	rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
fail() {
	echo "FAIL tinhub notify: $*"
	for f in a.log b.log deliveries mail watch.out watch2.out; do
		[ -f "$tmp/$f" ] && { echo "--- $f"; tail -n 20 "$tmp/$f"; }
	done
	exit 1
}

"$node" setup > /dev/null
secret=0123456789abcdef-test-secret
"$hooks" -secret "$secret" -fail 2 -out "$tmp/deliveries" -mail "$tmp/mail" > "$tmp/ports" 2> "$tmp/hooks.err" &
pids="$pids $!"
for i in $(seq 1 100); do [ "$(wc -l < "$tmp/ports")" -ge 2 ] && break; sleep 0.1; done
hport=$(sed -n 's/^http //p' "$tmp/ports")
sport=$(sed -n 's/^smtp //p' "$tmp/ports")
[ -n "$hport" ] && [ -n "$sport" ] || fail "the receiver did not start"
export TINHUB_TEST_ROUTES=1 TINHUB_TEST_SMTP=127.0.0.1:$sport
a=127.0.0.1:18441
b=127.0.0.1:18442
NODE_WORKER=1 "$node" serve "$a" > "$tmp/a.log" 2>&1 &
pids="$pids $!"
"$node" serve "$b" > "$tmp/b.log" 2>&1 &
pids="$pids $!"
for n in "$a" "$b"; do
	up=0
	for i in $(seq 1 40); do
		curl -sf "http://$n/api/v1/subscriptions" -H 'X-Test-User: 1' > /dev/null 2>&1 && { up=1; break; }
		sleep 0.25
	done
	[ $up = 1 ] || fail "node $n did not start"
done

# call USER METHOD PATH [BODY]: the status, with the answer in $tmp/out.json
call() {
	if [ $# -ge 4 ]; then
		curl -s -o "$tmp/out.json" -w '%{http_code}' -X "$2" -H "X-Test-User: $1" -H 'Content-Type: application/json' --data "$4" "http://$a$3"
	else
		curl -s -o "$tmp/out.json" -w '%{http_code}' -X "$2" -H "X-Test-User: $1" "http://$a$3"
	fi
}
expect() {
	[ "$1" = "$2" ] || fail "$3: status $1, not $2: $(cat "$tmp/out.json")"
}
hooksPath=/api/v1/repos/ada/tin/hooks

# webhooks: only the repository's admin manages them
expect "$(call 0 POST $hooksPath '{"url":"http://127.0.0.1:1/ok"}')" 401 "anonymous create"
expect "$(call 2 POST $hooksPath '{"url":"http://127.0.0.1:1/ok"}')" 403 "bob's create"
expect "$(call 1 POST $hooksPath '{"url":"ftp://127.0.0.1/x","secret":"'$secret'"}')" 422 "an ftp URL"
expect "$(call 1 POST $hooksPath '{"url":"http://127.0.0.1:1/ok","secret":"short"}')" 422 "a short secret"
expect "$(call 1 POST $hooksPath '{"url":"http://127.0.0.1:'$hport'/ok","secret":"'$secret'","kinds":["push"]}')" 201 "create ok"
jq -e '.id > 0 and .active == true and .kinds == ["push"] and .secret == "'$secret'"' "$tmp/out.json" > /dev/null || fail "create's answer: $(cat "$tmp/out.json")"
okId=$(jq -r .id "$tmp/out.json")
expect "$(call 1 POST $hooksPath '{"url":"http://127.0.0.1:'$hport'/flaky","secret":"'$secret'"}')" 201 "create flaky"
flakyId=$(jq -r .id "$tmp/out.json")
expect "$(call 1 POST $hooksPath '{"url":"http://127.0.0.1:'$hport'/down","secret":"'$secret'","kinds":["push"],"active":false}')" 201 "create down"
downId=$(jq -r .id "$tmp/out.json")
expect "$(call 1 POST $hooksPath '{"url":"http://127.0.0.1:'$hport'/ok","secret":"'$secret'","kinds":["review.landed"]}')" 201 "create other kind"
expect "$(call 1 PATCH $hooksPath/$downId '{"active":true}')" 200 "patch down"
jq -e '.active == true and .url == "http://127.0.0.1:'$hport'/down"' "$tmp/out.json" > /dev/null || fail "patch's answer: $(cat "$tmp/out.json")"
expect "$(call 1 GET $hooksPath)" 200 "list"
jq -e '(.hooks | length) == 4 and all(.hooks[]; has("secret") | not)' "$tmp/out.json" > /dev/null || fail "list: $(cat "$tmp/out.json")"
expect "$(call 2 GET $hooksPath)" 403 "bob's list"
expect "$(call 1 GET $hooksPath/999)" 404 "a missing webhook"
expect "$(call 1 DELETE $hooksPath/999)" 404 "delete a missing webhook"
echo "PASS tinhub notify: webhooks are managed by the repository's admin"

# subscriptions: bob follows the repository
expect "$(call 0 PUT /api/v1/repos/ada/tin/subscription)" 401 "anonymous follow"
expect "$(call 2 PUT /api/v1/repos/ada/tin/subscription)" 200 "bob follows ada/tin"
expect "$(call 2 PUT /api/v1/repos/ada/tin/changes/knope/subscription)" 404 "follow a change that does not exist"
expect "$(call 2 GET /api/v1/subscriptions)" 200 "bob's subscriptions"
jq -e '.subscriptions == [{"repo":"ada/tin","change":""}]' "$tmp/out.json" > /dev/null || fail "subscriptions: $(cat "$tmp/out.json")"
echo "PASS tinhub notify: subscriptions"

# live: websockets on node B follow the repository and one change; the event is committed through node A
"$node" watch "ws://$b/api/v1/live?topics=repo:ada/tin" 1 20 > "$tmp/watch.out" 2>&1 &
w1=$!
pids="$pids $w1"
"$node" watch "ws://$b/api/v1/live?topics=change:ada/tin/kwatched,repo:ada/nope" 1 20 > "$tmp/watch2.out" 2>&1 &
w2=$!
pids="$pids $w2"
for i in $(seq 1 40); do
	grep -q '"op":"subscribed"' "$tmp/watch.out" 2>/dev/null && grep -q '"op":"subscribed"' "$tmp/watch2.out" 2>/dev/null && break
	sleep 0.25
done
grep -q '"op":"error","topic":"repo:ada/nope"' "$tmp/watch2.out" || fail "a topic of a repository that does not exist is refused"
curl -sf -X POST "http://$a/test/event?repo=1&change=kother" > /dev/null || fail "an event through node A"
curl -sf -X POST "http://$a/test/event?repo=1&change=kwatched" > "$tmp/event.id" || fail "an event through node A"
wait $w1 || fail "the repository's websocket on node B saw no event"
wait $w2 || fail "the change's websocket on node B saw no event"
grep '"op":"event"' "$tmp/watch.out" | head -n 1 | jq -e '.topic == "repo:ada/tin" and .kind == "push" and .payload.changes[0].change == "kother"' > /dev/null || fail "the repository's event: $(cat "$tmp/watch.out")"
grep '"op":"event"' "$tmp/watch2.out" | jq -e '.topic == "change:ada/tin/kwatched" and .id == '"$(cat "$tmp/event.id")"' and .payload.changes[0].change == "kwatched"' > /dev/null || fail "the change's event: $(cat "$tmp/watch2.out")"
echo "PASS tinhub notify: an event committed through node A reaches websockets on node B"

# webhooks: two push events were fanned out by node A's worker to the three push webhooks (not to the review one)
for i in $(seq 1 120); do
	[ "$(grep -c '^/down ' "$tmp/deliveries" 2>/dev/null || true)" -ge 6 ] && [ "$(grep -c '^/flaky 200' "$tmp/deliveries" 2>/dev/null || true)" -ge 2 ] && break
	sleep 0.25
done
[ "$(grep -c '^/ok 200 sig=ok event=push' "$tmp/deliveries")" = 2 ] || fail "the receiver did not verify two deliveries to /ok"
grep -q 'sig=bad' "$tmp/deliveries" && fail "a delivery's signature did not verify"
grep -q 'json=false' "$tmp/deliveries" && fail "a delivery's body is not the event's JSON"
[ "$(grep -c '^/flaky 500' "$tmp/deliveries")" = 2 ] || fail "/flaky's two 500s"
[ "$(grep -c '^/flaky 200' "$tmp/deliveries")" = 2 ] || fail "/flaky was not tried again after its 500s"
grep -q '^/flaky 500 .* attempt=1' "$tmp/deliveries" || fail "/flaky's first attempt"
[ "$(grep -c '^/down 500' "$tmp/deliveries")" = 6 ] || fail "/down was not tried 3 times for each event"
sleep 2
[ "$(grep -c '^/down ' "$tmp/deliveries")" = 6 ] || fail "/down was tried after its third attempt"
"$node" states webhook > "$tmp/states"
grep -qx 'dead 2' "$tmp/states" || fail "the deliveries to /down are not given up: $(cat "$tmp/states")"
grep -qx 'done 4' "$tmp/states" || fail "the deliveries to /ok and /flaky are not done: $(cat "$tmp/states")"
expect "$(call 1 GET "$hooksPath/$downId/deliveries")" 200 "/down's deliveries"
jq -e '(.deliveries | length) == 6 and all(.deliveries[]; .status == 500) and ([.deliveries[].attempt] | sort) == [1,1,2,2,3,3]' "$tmp/out.json" > /dev/null || fail "/down's deliveries: $(cat "$tmp/out.json")"
expect "$(call 1 GET "$hooksPath/$okId/deliveries?limit=1")" 200 "/ok's deliveries"
jq -e '(.deliveries | length) == 1 and .deliveries[0].status == 200 and .next != ""' "$tmp/out.json" > /dev/null || fail "/ok's deliveries: $(cat "$tmp/out.json")"
echo "PASS tinhub notify: webhooks are signed, retried on a 500 and given up after 3 attempts"

# mail: bob follows ada/tin and is mailed about each push, once
for i in $(seq 1 40); do
	[ "$(grep -c '^MAIL ' "$tmp/mail" 2>/dev/null || true)" -ge 2 ] && break
	sleep 0.25
done
[ "$(grep -c '^MAIL FROM=tinhub@example.com TO=bob@example.com$' "$tmp/mail")" = 2 ] || fail "bob was not mailed twice"
grep -q '^Subject: \[ada/tin\] ada@example.com pushed to ada/tin' "$tmp/mail" || fail "the mail's subject"
grep -q 'TO=ada@example.com' "$tmp/mail" && fail "ada was mailed without following"
"$node" states notify | grep -qx 'done 2' || fail "the notify jobs are not done"
echo "PASS tinhub notify: followers are mailed"

expect "$(call 1 DELETE "$hooksPath/$flakyId")" 204 "delete flaky"
expect "$(call 2 DELETE /api/v1/repos/ada/tin/subscription)" 204 "bob unfollows"
expect "$(call 2 GET /api/v1/subscriptions)" 200 "bob's subscriptions"
jq -e '.subscriptions == []' "$tmp/out.json" > /dev/null || fail "after unfollowing: $(cat "$tmp/out.json")"
echo "PASS tinhub notify: delete"
