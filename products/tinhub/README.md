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
tinhub packs backup|restore DIR [--config FILE]            # the store's objects into DIR, or back (deploy/backup.sh)
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
`secrets.cookie`, the same on every node, and a bucket: content lives in S3-compatible object storage (R2, S3, MinIO)
in every deployment, one server included, and `tinhub run` stops at start when it cannot read the bucket.

```sh
db.addr = 127.0.0.1:5432
db.user = tinhub
db.password = file:/run/secrets/tinhub-db
secrets.nonce = file:/run/secrets/tinhub-nonce
secrets.cookie = file:/run/secrets/tinhub-cookie
s3.endpoint = https://ACCOUNT_ID.r2.cloudflarestorage.com
s3.region = auto
s3.bucket = tinhub
s3.access_key = R2_ACCESS_KEY_ID
s3.secret_key = file:/run/secrets/tinhub-s3
packs.dir = /var/lib/tinhub
```

For development, `products/tinhub/dev/local-s3.sh` starts a local S3-compatible store (MinIO, built from the version CI
pins) and prints the `TINHUB_S3_*` variables, so a laptop runs tinhub on object storage the way production does.
`packs.store = dir` keeps content on local disk at `packs.dir` instead; it is for tests and quick experiments only.

## Packs

Packs, their indexes, replay capsules and review diffs live in the pack store: a bucket (`packs.store = s3`, the
default), with each node's copies of what it read or wrote under `packs.dir/cache`. Postgres holds the pointers and
the state: whether a pack is pending, live or retired is its row. Refs and change versions are
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

Phase 1 is one Linux server with every role, Postgres beside it and content in a bucket (R2): `deploy/` has the systemd units,
a container image and compose file, the nightly backup and the restore, and the mirror sync that keeps the Tin repo a
read-only copy of GitHub. Follow [deploy/RUNBOOK.md](deploy/RUNBOOK.md) for install, upgrade, secrets and restore.

## Replay

`tit replay push` uploads sealed capsules to `POST /<owner>/<repo>/tit/v1/replay` (signed; write access or the replay
permission). tinhub keeps them in the pack store under `repos/<id>/capsules/<id>.tcap`, holds no key and never opens
them; capsules with the same panic and route form a failure group. Reading capsules takes the replay permission: an
admin of the repository, or a replay grant. Capsules expire after the repository's retention (30 days unless set);
the `replay.retention` job deletes them nightly. The API, under `/api/v1/repos/<owner>/<repo>/replay`:

- `GET groups` (`?state=open|closed`, `?cursor=`, `?limit=`), `GET groups/<id>`, `DELETE groups/<id>`
- `GET capsules` (`?group=`), `GET capsules/<id>`, `GET capsules/<id>/download` (the sealed bytes), `DELETE capsules/<id>`
- `GET retention`, `PUT retention` (`{"days": N}`, admin)
- `PUT grants/<user>`, `DELETE grants/<user>` (admin)

## Runner: a change's behaviour

The `runner` role replays a repository's capsules against a change version's build and its base's, every build and
replay in `packages/sandbox` (no network but the replay's own loopback, sources read-only, a lower CPU weight with
`runner.cgroup`), and groups what differs by the first effect that does: one new SQL query is one "different calls at
effect N" group. Only for owners that opt in, and only capsules sealed for the runner's key. Set it up:

```sh
tin replay key /run/secrets/tinhub-runner           # the runner's key pair; the public key is printed
runner.key = file:/run/secrets/tinhub-runner         # in tinhub.conf, with runner.tin = the Tin toolchain's root
```

then an org owner `PUT /api/v1/orgs/<org>/runner` (it answers the `public_key` the org's servers add to
`TIN_REPLAY_RECIPIENTS`), and a repository's admin sets what to build and the replayed program's environment:
`PUT /api/v1/repos/<owner>/<repo>/runner` with `{"entry": "main.tin", "sample": 20, "env": ["PAYMENTS_URL=…"]}`.

- `POST /api/v1/repos/<owner>/<repo>/changes/<change>/behaviour` (`{"version": N}`, else the newest) queues a run (409
  when the owner has not opted in); reviews queue one per new version (`runner.Request`).
- `GET …/changes/<change>/behaviour` (`?version=N`): the run and its groups (`calls`, `error` for a new 5xx, panic or
  timeout, `body`, `same`, `skipped`), each with its count, label, first differing effect and up to 20 capsule ids.

Both take read access and the replay permission.

**Replay checks: does my fix handle what happened?** On a failure group's page, "Check a fix" takes any branch, tag, change
or commit; the runner builds it and replays the group's capsules (or one of them) against it, each judged against its
own recording: `passed` (production's calls in the same order, no panic, below 500), `diverged` (a different call at
effect N, one more, or one fewer: the replay cannot judge past it), `panicked`, `failing` (a 5xx), `timeout` or
`skipped`, each with an explanation of why. Same opt-in, key and sandbox as runs.

- `POST /api/v1/repos/<owner>/<repo>/replay/checks` (`{"target": "fix/x", "group": ID, "capsule": ID}`, capsule
  optional) queues one: 202, 404 for an unknown revision or group, 409 when the owner has not opted in.
- `GET …/replay/checks` (`?group=ID`, `?limit=N`, `?before=ID`): the checks, newest first, with their verdicts counted,
  20 to a page; `next` is the following page's `before` (0 on the last). `GET …/replay/checks/<id>`: each capsule's
  verdict, label and explanation.

`runner.sandbox = off` (default `on`) builds and replays as plain child processes, for a development machine with no
sandbox such as macOS: the replayed program then has the host's network and files. Never set it on a server.

Postgres keeps outcomes only, never a request, body or effect key; the
private key is written only into a run's sandbox directory and removed with it. See
[design/tinhub.md §12](../../design/tinhub.md#12-the-runner-a-changes-behaviour-against-productions-requests-1028).

## Benchmarks and releases

`tit bench record` keeps results in `.tit/bench/<name>.jsonl`; a repository publishes them by committing those files
under `.bench/` (the same name, the same lines). On every push the `bench` job reads `.bench/*.jsonl` from the pushed
branch tips and change versions into `bench_results` (each line once, with the machine's OS, architecture, CPU, cores
and kernel), then compares each change version with its base (its first parent): a Linux result more than 10% slower
than the base's newest on the same benchmark, unit, architecture and machine fails the version's `bench` check and
leaves a note on its review. A unit ending in `/s` is a rate (higher is faster); any other is a cost. macOS results
are kept and charted on their own, never compared with Linux ones.

```sh
tit bench record fib "$ms" --unit ms && mkdir -p .bench && cp .tit/bench/fib.jsonl .bench/
tit add .bench && tit commit --amend && tit push
```

- `GET /api/v1/repos/<owner>/<repo>/bench`: every benchmark; `GET …/bench/<name>?limit=N`: its series, one per machine
  and unit, Linux first, a macOS one marked `development`.
- `GET …/changes/<change>/bench?version=N`: the version against its base: each benchmark's base and value, `slower`
  (a fraction) and `regressed`, and the check's `state`.
- `GET …/releases?limit=&cursor=`, `GET …/releases/<tag>`: what `tit ship` made (annotated tags), newest first, with the
  changelog's changes and whether a key registered to the tagger's account signed it (`verified`, `signed_by`).

See [design/tinhub.md §13](../../design/tinhub.md#13-benchmark-history-and-releases-1029).

## Notifications, webhooks and live updates

- `GET|POST /api/v1/repos/<owner>/<repo>/hooks`, `GET|PATCH|DELETE …/hooks/<id>`: a repository's webhooks, for its
  admins (`{"url","secret","kinds":["push"],"active"}`; the secret, 16 to 200 bytes, is generated when left out and
  shown only by the create). Each delivery is a POST of `{"kind","repo","delivery","event"}` signed in
  `X-Tinhub-Signature-256: sha256=<hex HMAC-SHA256 of the body>`; a failed one is tried again with backoff, up to 8
  attempts. `GET …/hooks/<id>/deliveries?limit=&cursor=` lists the attempts, newest first.
- `GET /api/v1/subscriptions`, `PUT|DELETE /api/v1/repos/<owner>/<repo>/subscription` and
  `…/changes/<change>/subscription`: what the caller follows; followers are mailed through `smtp.addr`.
- `GET /api/v1/live?topics=repo:<owner>/<repo>,change:<owner>/<repo>/<change>`: a websocket; send
  `{"op":"subscribe"|"unsubscribe","topic":…}`, receive `{"op":"event","topic","id","kind","payload"}`. Any node
  serves any subscriber: each polls the events table.

## Symbols and review diffs

A push's fan-out queues an `index` job for each head it moved and change version it added (`index.EnqueueFor`) and a
`review.diff` job for each change version (`diffs.EnqueueFor`). The index runs `tinc -symbols -json` on the packages
that changed, in `packages/sandbox` (or, where the host cannot build one, a plain child process; logged), and keeps
rows only for current heads and open changes. A file the compiler cannot read is indexed from its text.

- `GET /api/v1/repos/{owner}/{repo}/symbols?q=&ref=&change=&limit=&cursor=`: the declarations at a head (`ref`,
  default the default branch) or a change's newest version whose names start with `q` (`kind:Name`, `Type.Method`).
- `GET /api/v1/search?q=fn:ReadFile`: a declaration by name at the default branch of every repository the caller can
  read.
- `GET /api/v1/repos/{owner}/{repo}/changes/{change}/diffs/{base|previous}?version=`: a version's kept diff against its
  base or the version before it (semantic: each changed declaration with its line diff; or `kind: lines` with the
  reason); 202 until the job has computed it.
- `GET /api/v1/repos/{owner}/{repo}/changes/{change}/overlaps`: the other open changes that touch the declarations the
  change's newest version touches, and those declarations.

`TINHUB_TEST_INDEX_TIN=1` adds the index of this repository to the Postgres tests (about a minute).

## Reviews and landing

Each change gets a review when its first version arrives in a push (the push's change versions) or on its first use
through the API. Under `/api/v1/repos/{owner}/{repo}`:

- `GET changes/{change}`: the change's versions, oldest first, and its full id (`change`): a unique prefix of four
  letters or more (tit and the web show twelve) names it too, and the web's review page then opens under the full id.

- `GET changes/{change}/review`: state (`open`, `approved`, `changes_requested`, `landed`, `abandoned`), target, newest
  version, the votes that count, the checks, and `blocked` (why it cannot land yet, `""` when it can).
  `PATCH` with `{"state": "abandoned"|"open"}` (the author or a writer).
- `GET|POST changes/{change}/approvals` (`{"vote": "approve"|"changes"}`): a writer's approve counts (not the author's),
  a writer's changes blocks.
- `GET|POST changes/{change}/comments` (`{"body", "file", "decl": "fn Lstat", "line_offset", "version", "parent"}`),
  `PATCH changes/{change}/comments/{id}` (`{"resolved": true}`): a comment stays on its declaration through rebases and
  reformats, and is `outdated` once the declaration is renamed or deleted.
- `GET|POST changes/{change}/checks` (`{"name", "state": "pending"|"success"|"failure"|"error", "url", "description"}`):
  posting takes a `Tit-Signature` by a writer's key (an unsigned POST answers 401 with a `nonce` to sign with).
- `POST changes/{change}/land` (`{"stack": true}` lands the changes below it first): each change is rebased onto the
  target on the server when needed (a conflict refuses it), and the target moves once per change.
- `GET reviews?state=` (`active` is every review not landed or abandoned), `GET|PUT review/settings` (`{"approvals": 1, "checks": ["ci"]}`, admin).

Review events (`review.opened`, `review.voted`, `review.comment`, `review.check`, `review.landed`, `review.state`) reach
webhooks, mail and the live websocket. See [design/tinhub.md §14](../../design/tinhub.md#14-reviews-comments-checks-and-landing-1025).

## The web UI

The same binary serves the pages: every page URL answers the app shell, which loads the UI (plain ES modules and CSS
under `web/static`, embedded at build time, no build step and no Node at run time) and reads and writes through the
JSON API. There is a page for every feature: the code and its history, changes and stacks, reviews with semantic diffs
and line comments, votes and landing, checks, benchmark history, releases, replay failure groups, activity, repository
and org settings, account settings and site administration. Sign-in is a key: the page shows a code, and
`tit login <site> --code CODE` approves it. `/` (or Ctrl+K) opens a palette that jumps to a repository, page or
command, a symbol after `#`, and on a repository's pages to a file by name (`t` opens it there too). See
[design/tinhub.md §15](../../design/tinhub.md#15-the-web-layer-1081).

To work on the UI against a running node, set `web.dir` to `products/tinhub/web/static`: files are read from disk on
every request. `web/dev/seed.sh` fills a fresh node with people, an org, repositories, a reviewed stack, benchmark
history and a release:

```sh
TINHUB_WEB_DIR=$PWD/products/tinhub/web/static tinhub run &
sh products/tinhub/web/dev/seed.sh path/to/tinhub path/to/tit   # then open the URL it prints
# with the runner on (TINHUB_RUNNER_KEY=file:KEY from `tin replay key KEY`, TINHUB_RUNNER_TIN=$PWD, and on macOS
# TINHUB_RUNNER_SANDBOX=off), the seed also records dev/shop's failures and checks five fix branches against them;
# SEED_ORDERS=1 with SEED_ORDERS_PG_ADDR/_USER/_PASSWORD/_DATABASE adds dev/orders (Postgres, Redis and a real website,
# so it needs the network): three failures and eight fix branches
node --test products/tinhub/web/test/*.test.js                 # the UI's unit tests
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
  sh products/tinhub/tests/run.sh               # plus Postgres: schema, event queue, accounts, readiness and drain
```

On Linux the Postgres checks also run the runner end to end (`tests/runner.sh`: examples/checkout.tin recorded, a change
adding a SQL query replayed in the sandbox), skipped where the sandbox cannot run (no user namespaces) or there is no
`redis-server` or docker. They also run benchmark history end to end (`tests/bench.sh`: the Tin repo's `bench/fib.tin`
timed and recorded with tit, a change made three times slower failing its check with a note, `tit ship`'s release).
With `TINHUB_TEST_REDIS=host:port` as well, the accounts tests also check that sessions survive a restart and that a
login code is claimed once (under a random key prefix).

`tests/web.sh` checks the shell, its security headers and the assets against a running node, and runs the UI's unit
tests when Node is installed. The test database is emptied by the tests. CI runs the Postgres checks on Linux x86-64 against the runner's
PostgreSQL, and the rest on every native job.
