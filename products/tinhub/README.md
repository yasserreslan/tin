# tinhub

tinhub hosts tit repositories: the tit protocol for clone, fetch and push, accounts and keys, reviews, and a JSON API,
written in Tin and run on Linux (arm64 and x86-64). The design, with every table, key and budget, is
[design/tinhub.md](../../design/tinhub.md); the work is milestone tinhub-backend-engine (umbrella #1030).

## Commands

```sh
tinhub run [--config FILE] [--roles node,worker,runner]   # serve; every role by default
tinhub migrate [--config FILE]                             # apply the pending migrations, then exit
tinhub version
```

`tinhub run` refuses to start until `tinhub migrate` has brought the database to the binary's schema. Two migrators
at once are safe: they take an advisory lock and apply each migration once.

## Configuration

A file of `key = value` lines (`--config`, else `$TINHUB_CONFIG`, else `/etc/tinhub/tinhub.conf`), and the
environment: `TINHUB_` and the key in upper case with dots as underscores (`db.password` is `TINHUB_DB_PASSWORD`). A
value `file:PATH` is read from that file, for secrets mounted as files. Every key and its default is in
[design/tinhub.md §3](../../design/tinhub.md#3-the-binary-and-its-configuration). A node needs `secrets.nonce` and
`secrets.cookie`, the same on every node.

```sh
db.addr = 127.0.0.1:5432
db.user = tinhub
db.password = file:/run/secrets/tinhub-db
secrets.nonce = file:/run/secrets/tinhub-nonce
secrets.cookie = file:/run/secrets/tinhub-cookie
packs.dir = /var/lib/tinhub
```

## Health and shutdown

- `GET /healthz`: 200 while the process runs.
- `GET /readyz`: 200 when Postgres answers and the schema is current, else 503 and why.
- `GET /metrics`: the event queue in Prometheus text (ready and dead jobs per kind, the oldest ready job's age), to
  loopback clients only.
- SIGTERM: the node stops accepting, requests in flight finish within `shutdown.deadline` (a push's deadline plus
  10 s by default), and the workers give their claimed jobs back to the queue.

Logs are herald lines; each request's line carries `req=` (its `X-Request-Id`, echoed in the answer).

## Tests

```sh
sh products/tinhub/tests/run.sh                 # unit tests and the build, on any platform
TINHUB_TEST_DB=127.0.0.1:5432/tinhub_test TINHUB_TEST_DB_USER=tin TINHUB_TEST_DB_PASSWORD=... \
  sh products/tinhub/tests/run.sh               # plus Postgres: schema, event queue, server readiness and drain
```

The test database is emptied by the tests. CI runs the Postgres checks on Linux x86-64 against the runner's
PostgreSQL, and the rest on every native job.
