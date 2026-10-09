# Shared by tinhub's process tests: a Redis of their own on 127.0.0.1:16379 (redis-server, else the redis image in
# docker), started and killed at will. start_redis fails (status 1) when neither is installed.
rpid=
container=
port=16379
stop_redis() {
	if [ -n "$rpid" ]; then kill -9 "$rpid" 2>/dev/null || true; wait "$rpid" 2>/dev/null || true; rpid=; fi
	if [ -n "$container" ]; then docker rm -f "$container" > /dev/null 2>&1 || true; container=; fi
}
start_redis() {
	if command -v redis-server > /dev/null 2>&1; then
		redis-server --port $port --bind 127.0.0.1 --save '' --appendonly no > "${tmp:-/tmp}/redis.log" 2>&1 & rpid=$!
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
