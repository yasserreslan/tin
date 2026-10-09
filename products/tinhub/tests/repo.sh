#!/bin/sh
# The repository's process tests (#1011, #1016) against TINHUB_TEST_DB. Usage: repo.sh REPO_DRIVER
#   - two processes (two nodes) race to move one ref: every move lands exactly once or is refused RefChanged;
#   - pushes read back through two nodes' Redis caches stay right while Redis is killed and comes back (needs
#     redis-server, or docker for the redis image);
#   - a push, a repack and a retire, killed at each crash point in turn (TIT_CRASH_AT), never leave main reaching a
#     missing pack, and the sweep removes what is left over.
set -eu
driver=$1
tmp=$(mktemp -d)
rpid=
container=
stop_redis() {
	if [ -n "$rpid" ]; then kill -9 "$rpid" 2>/dev/null || true; wait "$rpid" 2>/dev/null || true; rpid=; fi
	if [ -n "$container" ]; then docker rm -f "$container" > /dev/null 2>&1 || true; container=; fi
}
cleanup() {
	stop_redis
	rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM

"$driver" setup "$tmp/node" > /dev/null
"$driver" race 300 a > "$tmp/a" & a=$!
"$driver" race 300 b > "$tmp/b" & b=$!
wait $a
wait $b
cat "$tmp/a" "$tmp/b" | "$driver" checkrace

port=16379
start_redis() {
	if command -v redis-server > /dev/null 2>&1; then
		redis-server --port $port --bind 127.0.0.1 --save '' --appendonly no > "$tmp/redis.log" 2>&1 & rpid=$!
	elif command -v docker > /dev/null 2>&1; then
		container=tinhub-test-redis-$$
		image=redis:7-alpine
		docker pull -q "mirror.gcr.io/library/$image" > /dev/null 2>&1 && image=mirror.gcr.io/library/$image
		docker run -d --rm --name "$container" -p 127.0.0.1:$port:6379 "$image" > /dev/null
	else
		return 1
	fi
	for i in $(seq 1 50); do
		if [ -n "$container" ]; then
			docker exec "$container" redis-cli ping 2>/dev/null | grep -q PONG && return 0
		else
			redis-cli -p $port ping 2>/dev/null | grep -q PONG && return 0
		fi
		sleep 0.2
	done
	echo "FAIL tinhub: Redis did not start"; exit 1
}
if start_redis; then
	export TINHUB_TEST_REDIS=127.0.0.1:$port TINHUB_TEST_PREFIX="tinhub-test-$$:"
	"$driver" setup "$tmp/node" > /dev/null
	"$driver" redis 150 > "$tmp/redis.out" 2>&1 & d=$!
	sleep 1
	stop_redis
	sleep 1
	start_redis
	status=0
	wait $d || status=$?
	cat "$tmp/redis.out"
	[ $status = 0 ] || { echo "FAIL tinhub ref cache with Redis killed partway"; exit 1; }
	stop_redis
	unset TINHUB_TEST_REDIS TINHUB_TEST_PREFIX
else
	echo "SKIP tinhub ref cache with Redis killed partway (no redis-server or docker)"
fi

n=1
while :; do
	"$driver" setup "$tmp/node" > /dev/null
	status=0
	TIT_CRASH_AT=$n "$driver" crash "$tmp/node" > "$tmp/crash.out" 2>&1 || status=$?
	if [ $status != 0 ] && [ $status != 86 ]; then
		cat "$tmp/crash.out"; echo "FAIL tinhub crash at point $n: status $status"; exit 1
	fi
	"$driver" check "$tmp/node" > "$tmp/check.out" 2>&1 || { cat "$tmp/check.out"; echo "FAIL tinhub crash at point $n"; exit 1; }
	[ $status = 86 ] || break
	n=$((n + 1))
	[ $n -le 100 ] || { echo "FAIL tinhub crash: more than 100 crash points"; exit 1; }
done
echo "PASS tinhub crash points: $((n - 1)) crashes, each left the repository whole"
