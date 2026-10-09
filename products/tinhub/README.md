# tinhub

tinhub hosts tit repositories: the tit protocol for clone, fetch and push, accounts and keys, reviews, and a JSON API,
written in Tin and run on Linux (arm64 and x86-64). The design, with every table, key and budget, is
[design/tinhub.md](../../design/tinhub.md); the work is milestone tinhub-backend-engine (umbrella #1030).

## Commands

```sh
tinhub run [--config FILE] [--roles node,worker,runner]   # serve; every role by default
tinhub migrate [--config FILE]                             # apply the pending migrations, then exit
tinhub packs sweep [--config FILE]                         # delete what crashed pushes and repacks left, once
tinhub packs copy [--config FILE]                          # copy packs.dir into the s3.* bucket; rerun to catch up
tinhub check [--config FILE]                               # every live pack in the store; refs and packs fingerprints
tinhub admin invite EMAIL [--site-admin]                   # a one-use code for tit key add
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

## Packs

Packs live in the pack store (`packs.store = dir`, at `packs.dir`; or `s3`, a bucket with each node's copies under
`packs.dir/cache`); whether a pack is pending, live or retired is its row in Postgres. Refs and change versions are
rows too, moved by compare-and-swap in one transaction per push under the repository's lock, and cached in Redis when
`redis.addr` is set. Every `packs.sweep` the sweeps delete the packs crashed pushes left and those retired longer than
`packs.grace`. See [design/tinhub.md §7](../../design/tinhub.md#7-the-pack-store-1011-1015).

## Accounts and sign-in

`products/tinhub/accounts` holds users, orgs, members, teams, repositories, grants, keys, invites, login requests and
sessions. A user's role on a repository (read, write or admin) comes from owning it, owning its org, its own grant, its
teams' grants, and public visibility; site admins have admin everywhere. Every change to access writes an `audit_log`
row. There are no passwords: an account gets its first key with an invite (`tinhub admin invite`, then
`tit key add URL CODE`), and a browser signs in when `tit login --code CODE` approves its login request with a key.
Browser sessions live in Redis (`redis.addr`) behind a cookie signed with `secrets.cookie`. **Losing Redis, flushing it
or changing `secrets.cookie` signs everyone out**, and they sign in again with `tit login`; nothing else is lost.
Without `redis.addr` there are no browser sessions, and key-signed requests still work. See
[design/tinhub.md §11](../../design/tinhub.md#11-accounts-sign-in-and-sessions-1012).

## Deployment

Phase 1 is one Linux server with every role, Postgres beside it and packs on disk: `deploy/` has the systemd units,
a container image and compose file, the nightly backup and the restore, and the mirror sync that keeps the Tin repo a
read-only copy of GitHub. Follow [deploy/RUNBOOK.md](deploy/RUNBOOK.md) for install, upgrade, secrets and restore.

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
  sh products/tinhub/tests/run.sh               # plus Postgres: schema, event queue, accounts, readiness and drain
```

With `TINHUB_TEST_REDIS=host:port` as well, the accounts tests also check that sessions survive a restart and that a
login code is claimed once (under a random key prefix).

The test database is emptied by the tests. CI runs the Postgres checks on Linux x86-64 against the runner's
PostgreSQL, and the rest on every native job.
