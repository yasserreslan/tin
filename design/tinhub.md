# Interface: tinhub, hosting for tit repositories (layout, roles, tables, seams) — #1000

Status: **accepted** for the tinhub-backend-engine milestone (umbrella #1030). Every tinhub issue (#1005 – #1029)
builds on the names, tables and rules below; a change to one of them is made in this file first, in the PR that
needs it.

The overview and the reasons behind the choices are in the design page linked from #1030. This file fixes what the
issues share: the decisions, the layout of `products/tinhub`, the binary and its configuration, the Postgres tables,
the pack store's keys, the event queue and the budgets. tit's side of the seams (the protocol, `transport.Repo`,
replay capsules and sign-in) is design/tit.md §15 – §18; nothing here changes it.

---

## 1. The decisions

1. **Packs per repository, no deduplication across repositories.** A pack lives at
   `repos/<repo id>/packs/<hash>.pack` with its `.idx` beside it. A key shared by several repositories (a global
   content address) would let anyone who can push to one repository learn whether a private repository holds an
   object, by timing or by the absence of an upload. Storage is cheap; that leak is not.
2. **Sign-in with tit keys, never passwords.** `users` has no password hash. An account's first key comes with an
   invite (`tinhub admin invite`, then `tit key add`); a browser signs in with a login request that a registered key
   approves (design/tit.md §18). The design page's `seal.Pbkdf2Sha256` note is dropped.
3. **Refs, change versions and the repository lock live in Postgres from phase 1.** A push commits in one
   transaction under `pg_advisory_xact_lock(<repo id>)`: the pack goes live, refs move by compare-and-swap, the new
   change versions are inserted and a `push` event is queued. Phase 2 only adds nodes; no data moves out of files.
4. **Data first, pointer second.** A pack is written and fsynced (directory) or uploaded (S3) and recorded as
   `pending` before any ref can reach it; a ref moves only if it still holds the value the client saw.
5. **One binary, many roles.** `node` (the tit protocol, the JSON API, live updates, health), `worker` (the event
   queue's handlers) and `runner` (sandboxed replays, #1028) are the same program started with different roles.
6. **Postgres holds everything that changes, Redis only what can be rebuilt** (sessions, the ref cache, rate
   limits). Losing Redis signs everyone out and resets rate limits; nothing else.
7. **Linux arm64 and x86-64 only.** tinhub is built and run on Linux; on macOS it builds for development and runs
   the unit tests that need no Postgres.

### Cleanup

tit has no gc. What replaces the design page's "repack + gc" worker:

| what | when | issue |
|---|---|---|
| repack: many live packs into one, the old ones retired | a repository has more than `repack.packs` live packs | #1007, #1014 |
| prune: unreachable objects older than their retention, by rewriting packs | nightly | #1010, #1014 |
| purge: an object removed on request (an admin, audited) | on request | #1010, #1014 |
| sweep of pending packs: deleted when older than the push deadline plus a margin | every `packs.sweep` | #1011 |
| sweep of retired packs: deleted after the grace period (never shorter than the longest fetch) | every `packs.sweep` | #1007, #1011 |

---

## 2. Where things live

```text
products/tinhub/main.tin          the tinhub command: run, migrate, version, admin, packs (#1005)
products/tinhub/config/           the configuration: file, environment, secrets (#1005)
products/tinhub/db/               the migrations (embedded), the migrator and typed query helpers (#1008)
products/tinhub/packs/            the PackStore shape, the directory store (#1011) and the S3 store (#1015)
products/tinhub/repo/             transport.Repo on Postgres and a PackStore (#1016)
products/tinhub/proto/            tit's protocol mounted for every repository (#1018)
products/tinhub/accounts/         users, orgs, teams, keys, invites, sessions and access (#1012)
products/tinhub/api/              the JSON API under /api/v1 (#1019)
products/tinhub/events/           the event queue and the worker loop (#1013)
products/tinhub/workers/          the job handlers: repack, prune, purge, mirror, index, diffs (#1014, #1020, #1021)
products/tinhub/replay/           the capsule store and failure groups (#1022)
products/tinhub/live/             notifications, webhooks and the websocket (#1023)
products/tinhub/review/           reviews, comments, checks and landing (#1025)
products/tinhub/runner/           the runner role (#1028)
products/tinhub/deploy/           systemd unit, container image, backup and restore, runbook (#1024, #1026)
products/tinhub/tests/run.sh      the tinhub checks (#1005)
```

Each directory is one package imported as `"../<part>"` from a sibling and `"./<part>"` from `main.tin`, as tit's
are. tinhub imports tit's packages (`object`, `store`, `packfile`, `change`, `transport`, `semantic`, `diff`,
`revwalk`) as `"../../tit/<part>"`; they keep no process-wide state (design/tit.md §16).

| package | owns | capabilities (`tin caps`) |
|---|---|---|
| `config` | `Config`, loading and checking it | files |
| `db` | migrations, `Db` (the Postgres clients), query helpers | net |
| `packs` | `PackStore`, `DirStore`, `S3Store`, the cache | files, net |
| `repo` | `PgRepo` (a `transport.Repo`) | net, files |
| `accounts` | access checks, keys, invites, sign-in, sessions, audit | net (files only through tit's `transport`, for its wire types) |
| `events` | `Queue`, `Handler`, the worker loop | net |

---

## 3. The binary and its configuration

```text
tinhub run [--config FILE] [--roles node,worker,runner]   serve; all roles by default
tinhub migrate [--config FILE]                             apply the pending migrations, then exit
tinhub version [-v]                                        the version, and with -v the build's Tin version
tinhub admin invite --email ADDRESS [--org NAME] [--admin] a one-use invite code for tit key add (§7)
tinhub packs copy [--config FILE]                          copy every pack under packs.dir into the s3.* bucket, idempotently
tinhub packs sweep [--config FILE]                         run the pending and retired sweeps once
```

**Configuration** is a file of `key = value` lines (`#` starts a comment, blank lines are ignored; a value may be
quoted with `"`), read from `--config`, else `$TINHUB_CONFIG`, else `/etc/tinhub/tinhub.conf` when it exists. Every
key can be set or overridden by the environment: `TINHUB_` and the key in upper case with `.` as `_`
(`db.password` is `TINHUB_DB_PASSWORD`). A value of the form `file:PATH` is read from that file (trailing newline
removed), which is how a secret comes from a mounted file. Unknown keys are refused at start.

| key | default | meaning |
|---|---|---|
| `roles` | `node,worker,runner` | the roles this process runs |
| `listen` | `:8080` | the node's address |
| `tls.cert`, `tls.key` | none | PEM files; set both to serve HTTPS (TLS 1.3, HTTP/2). Reloaded on SIGHUP |
| `public_url` | `http://localhost:8080` | the URL clients use, for links and login answers |
| `db.addr`, `db.user`, `db.name` | `127.0.0.1:5432`, `tinhub`, `tinhub` | the Postgres primary |
| `db.password` | none | **secret** |
| `db.replica` | none | a read replica's address (phase 2): stale-tolerant API reads go there |
| `db.max_total` | `16` | the connection cap across all cores |
| `redis.addr` | none | Redis; without it sessions and rate limits stay per process and the ref cache is off |
| `packs.store` | `dir` | `dir` or `s3` |
| `packs.dir` | `/var/lib/tinhub` | the directory store's root (`dir`), or the cache's root (`s3`) |
| `packs.cache` | `10gb` | the node's pack cache size (`s3`) |
| `packs.grace` | `1h` | how long a retired pack stays readable |
| `packs.sweep` | `10m` | how often the sweeps run |
| `s3.endpoint`, `s3.region`, `s3.bucket` | none, `auto`, none | the object store (phase 2) |
| `s3.access_key` | none | the access key id |
| `s3.secret_key` | none | **secret** |
| `s3.path_style` | `true` | path-style addressing (MinIO); `false` for virtual-hosted buckets |
| `secrets.nonce` | none | **secret**, required: signs protocol nonces; the same on every node |
| `secrets.cookie` | none | **secret**, required: signs session cookies; the same on every node |
| `push.deadline` | `60s` | a push's `within` |
| `push.memory` | `512mb` | a push's `limit memory` |
| `repo.quota` | `10gb` | the default size quota of a repository |
| `limits.pack` | `2gb` | the largest pack a push may carry |
| `limits.refs` | `10000` | the most refs a push may move |
| `workers.concurrency` | `4` | jobs one worker process runs at once |
| `workers.lease` | `5m` | how long a claimed job stays claimed without a heartbeat |
| `repack.packs` | `16` | the live pack count that queues a repack |
| `api.rate` | `600` | API requests per minute for each user (signed in) or address; `0` for no limit |
| `smtp.addr`, `smtp.from` | none | mail for notifications; none sends no mail |
| `log.level` | `info` | `debug`, `info`, `warn`, `error` |

Secrets are `secret str` from the moment they are read: `say`, faults and logs cannot show them.

**Roles at start.** Every role first checks that the schema is current (`tinhub migrate` has run) and refuses to start
otherwise; migrations are never applied implicitly by `run`.

- `node`: anvil on `listen` (h2c, or HTTPS and h2 with `tls.*`) with the routes of §4.
- `worker`: `workers.concurrency` claim loops over the event queue (§6) on core 0's tasks, and the sweeps.
- `runner`: the runner loop (#1028), which starts every replay through `sandbox`.

**Health.** `GET /healthz` answers 200 `ok` while the process runs. `GET /readyz` answers 200 `ready` when Postgres
answers and the newest applied migration is the newest the binary knows, else 503 with the reason.

**Shutdown.** On SIGTERM a node stops accepting, lets in-flight requests finish within `shutdown.deadline` (default
`push.deadline` plus 10 s, so a push in flight completes), and a worker stops claiming, finishes or gives back (its
lease cleared) every job it holds. Then the process exits 0.

**Logs.** `herald` lines; every request's lines carry `req=<id>` (the `X-Request-Id` header when it is a safe token,
else 16 random hex digits, echoed in the answer).

---

## 4. Routes

| route | role | issue |
|---|---|---|
| `GET /healthz`, `GET /readyz`, `GET /metrics` (loopback only) | every node | #1005, #1013 |
| `GET /<owner>/<repo>/tit/v1/heads`, `POST …/fetch`, `POST …/push` | node | #1018 |
| `POST /<owner>/<repo>/tit/v1/replay`, `GET …/replay`, `GET …/replay/<id>` | node | #1022 |
| `POST /tit/v1/login`, `GET\|POST /tit/v1/login/<code>`, `POST /tit/v1/keys` | node | #1012 |
| `/api/v1/…` (JSON) | node | #1019 and later |
| `GET /api/v1/live` (websocket) | node | #1023 |

Owner names (users and orgs share one namespace) and repository names match `[a-z0-9][a-z0-9-]{0,38}` and
`[A-Za-z0-9._-]{1,100}` (not `.` or `..`, not ending in `.tit`). A private repository answers 404, never 403, to
anyone who cannot read it, on every route.

---

## 5. Postgres tables

Migration `0001` creates the tables below; later issues add migrations (`NNNN_<name>.sql`, forward only). Ids are
`bigint generated always as identity`; times are `timestamptz`; object ids and pack hashes are lower-case hex
`text` (64 digits), change ids tit's letters (`text`). Every foreign key is `on delete cascade` unless noted;
repositories and accounts are soft-deleted (`deleted_at`), so cascades run only when a purge removes them for good.

### Accounts and access (#1012)

| table | columns | constraints |
|---|---|---|
| `owners` | `id`, `name text`, `kind text` (`user`, `org`), `created_at` | `unique (name)`; the namespace of `/<owner>/…` |
| `users` | `id`, `owner_id`, `email text`, `display text`, `site_admin bool`, `created_at`, `deleted_at` | `unique (owner_id)`, `unique (lower(email))`; **no password** |
| `orgs` | `id`, `owner_id`, `display text`, `created_at`, `deleted_at` | `unique (owner_id)` |
| `members` | `org_id`, `user_id`, `role text` (`member`, `owner`), `created_at` | `primary key (org_id, user_id)` |
| `teams` | `id`, `org_id`, `name text`, `created_at` | `unique (org_id, name)` |
| `team_members` | `team_id`, `user_id` | `primary key (team_id, user_id)` |
| `grants` | `id`, `repo_id`, `user_id`, `team_id`, `role text` (`read`, `write`, `admin`) | exactly one of `user_id`, `team_id`; `unique (repo_id, user_id)`, `unique (repo_id, team_id)` |
| `keys` | `id`, `user_id`, `public_key text` (base64, 32 bytes), `name text`, `created_at`, `last_used_at`, `revoked_at` | `unique (public_key)` |
| `invites` | `code text`, `email text`, `org_id` (nullable), `site_admin bool`, `created_by` (nullable), `created_at`, `expires_at`, `used_at`, `used_by` | `primary key (code)` |
| `login_requests` | `code text`, `state text` (`pending`, `approved`, `used`), `user_id` (nullable), `created_at`, `expires_at`, `approved_at` | `primary key (code)` |
| `audit_log` | `id`, `at`, `actor_id` (nullable, `on delete set null`), `action text`, `target_kind text`, `target_id bigint`, `detail jsonb`, `ip text` | index `(target_kind, target_id, at)` |

A repository's role for a user is the highest of: `admin` when the user owns it or is an owner of its org, the user's
grant, the grants of the user's teams, and `read` when it is public. A site admin has `admin` everywhere.

### Repositories, refs and packs (#1011, #1016)

| table | columns | constraints |
|---|---|---|
| `repos` | `id`, `owner_id`, `name text`, `visibility text` (`public`, `private`), `description text`, `default_branch text`, `quota_bytes bigint`, `size_bytes bigint`, `push_seq bigint` (the last version operation number), `created_at`, `deleted_at` | `unique (owner_id, lower(name)) where deleted_at is null` |
| `refs` | `repo_id`, `name text`, `target text` (as a ref file writes it: hex, or `change <letters>`), `updated_at` | `primary key (repo_id, name)` |
| `changes` | `repo_id`, `change_id text`, `version int` (1, 2, …), `commit_id text`, `op bigint` (the push's `push_seq`), `pushed_by` (nullable, `on delete set null`), `created_at` | `primary key (repo_id, change_id, version)`, `unique (repo_id, change_id, op)` |
| `packs` | `repo_id`, `hash text`, `state text` (`pending`, `live`, `retired`), `size_bytes bigint`, `objects bigint`, `staged_at`, `live_at`, `retired_at` | `primary key (repo_id, hash)`, check that the times match the state, index `(state, staged_at)` |

A version's operation number is `repos.push_seq`, incremented in the push transaction: it orders one change's
versions (design/tit.md §16).

### Reviews (#1025)

| table | columns | constraints |
|---|---|---|
| `reviews` | `repo_id`, `change_id text`, `state text` (`open`, `approved`, `changes_requested`, `landed`, `abandoned`), `target text` (the ref it lands on), `opened_by`, `created_at`, `updated_at`, `landed_at` | `primary key (repo_id, change_id)` |
| `approvals` | `repo_id`, `change_id`, `version int`, `user_id`, `vote text` (`approve`, `changes`), `created_at` | `primary key (repo_id, change_id, version, user_id)` |
| `comments` | `id`, `repo_id`, `change_id text`, `version int`, `author_id`, `parent_id` (nullable), `decl text` (the declaration's qualified name, `""` for a file comment), `file text`, `line_offset int` (lines from the declaration's first line), `body text`, `outdated bool`, `created_at`, `resolved_at` | index `(repo_id, change_id)` |
| `checks` | `repo_id`, `change_id text`, `version int`, `name text`, `state text` (`pending`, `success`, `failure`, `error`), `url text`, `key_id`, `updated_at` | `primary key (repo_id, change_id, version, name)` |

### Workers' results (#1020, #1021, #1029)

| table | columns | constraints |
|---|---|---|
| `symbols` | `repo_id`, `commit_id text`, `package text`, `name text`, `kind text`, `file text`, `line int`, `end_line int` | index `(repo_id, commit_id)`, index `(lower(name))` |
| `change_overlaps` | `repo_id`, `change_id text`, `version int`, `other_change_id text`, `decl text`, `created_at` | `primary key (repo_id, change_id, version, other_change_id, decl)` |
| `diffs` | `repo_id`, `change_id text`, `version int`, `against text` (`base`, `previous`), `kind text` (`semantic`, `lines`), `body text` (capped; a large one is `""` and stored in the pack store), `stored bool` | `primary key (repo_id, change_id, version, against)` |
| `bench_results` | `id`, `repo_id`, `commit_id text`, `name text`, `value double precision`, `unit text`, `os text`, `arch text`, `cpu text`, `kernel text`, `machine text`, `created_at` | index `(repo_id, name, created_at)` |

### Replay (#1022)

| table | columns | constraints |
|---|---|---|
| `capsules` | `repo_id`, `id text` (hex SHA-256 of the bytes), `group_id text`, `commit_id text`, `name text`, `signer text`, `summary jsonb`, `status int`, `route text`, `size_bytes bigint`, `created_at`, `expires_at` | `primary key (repo_id, id)`, index `(expires_at)` |
| `replay_groups` | `repo_id`, `id text`, `panic text`, `route text`, `state text` (`open`, `closed`), `fixed_by text` (a change id), `reopened_by text`, `count bigint`, `first_at`, `last_at` | `primary key (repo_id, id)` |

### Events, webhooks (#1013, #1023)

| table | columns | constraints |
|---|---|---|
| `events` | `id`, `kind text`, `repo_id` (nullable), `payload jsonb`, `state text` (`ready`, `claimed`, `done`, `dead`), `attempts int`, `next_at`, `lease_until`, `claimed_by text`, `last_error text`, `created_at`, `done_at` | index `(next_at) where state = 'ready'`, index `(lease_until) where state = 'claimed'` |
| `webhooks` | `id`, `repo_id`, `url text`, `secret text`, `kinds text[]`, `active bool`, `created_by`, `created_at` | |
| `deliveries` | `id`, `webhook_id`, `event_id`, `attempt int`, `status int`, `error text`, `duration_ms int`, `created_at` | index `(webhook_id, created_at)` |
| `schema_migrations` | `version int`, `name text`, `applied_at` | `primary key (version)` |

### Mirrors (#1014, migration `0002_workers`)

| table | columns | constraints |
|---|---|---|
| `mirrors` | `repo_id`, `url text` (`https://…`; a local git directory only where the worker allows it), `token text` (HTTP basic password, `''` for none), `active bool`, `created_at`, `mirrored_at`, `last_error text` | `primary key (repo_id)`; set by a repository's admins |

---

## 6. The event queue (#1013)

An event is a row of `events`, inserted in the transaction that made the change it describes (a push inserts `push`
in the push transaction). Workers run handlers by `kind`; a job is an event in the hands of a worker.

- **Claim**: `UPDATE events SET state = 'claimed', lease_until = now() + lease, claimed_by = $me, attempts =
  attempts + 1 WHERE id IN (SELECT id FROM events WHERE state = 'ready' AND next_at <= now() ORDER BY next_at LIMIT
  $n FOR UPDATE SKIP LOCKED) RETURNING …`. A claimed job whose `lease_until` passed is `ready` again (the reaper runs
  with each claim). A worker extends the lease of a long job.
- **Finish**: `done` (with `done_at`), or on a fault `ready` with `next_at = now() + backoff(attempts)` (1 s doubling to
  10 min, with jitter) and `last_error`; after `attempts` reaches the kind's maximum (default 10), `dead`.
- **Delivery is at least once**: handlers are idempotent (they key their writes by the event and its repository).
- **Wake-up**: polling every second when idle (and `LISTEN/NOTIFY` once the postgres package has it, #1013).
- **Budgets**: each job runs inside `within` and `limit memory, tasks` of its kind; over budget, the job fails alone.

| kind | queued by | handler | budget |
|---|---|---|---|
| `push` | a push | fan-out: index, diffs, overlap, notify, webhooks, mirror, repack check | 30s, 32mb |
| `index` | `push` | symbols for moved heads and new versions (#1020) | 2m, 256mb |
| `review.diff` | `push` | semantic diffs of a new version (#1021) | 5s, 48mb |
| `overlap` | `push` | overlaps of a new version (#1021) | 10s, 32mb |
| `repack` | `push` when live packs exceed `repack.packs` | #1014 | 10m, 512mb |
| `prune` | nightly | #1014 | 30m, 512mb |
| `purge` | an admin | #1014 | 30m, 512mb |
| `mirror` | `push` | #1014 | 10m, 256mb |
| `notify`, `webhook` | events with subscribers | #1023 | 30s, 16mb |
| `replay.retention` | nightly | #1022 | 5m, 64mb |
| `bench` | `push` | #1029 | 1m, 64mb |

Metrics: `GET /metrics` (Prometheus text, answered to loopback clients only): ready and dead jobs per kind, claimed
jobs, and the age of the oldest ready job of each kind.

**The repository workers** (`products/tinhub/workers`, #1014): `workers.Handlers(deps)` gives the handlers of `push`,
`repack`, `prune`, `purge` and `mirror` with the budgets above.

- `push` queues, in one transaction, the follow-up kinds (`followUps` in `workers/push.tin` is the one place each of
  index, review.diff, overlap, notify, webhook and bench is added once its handler exists, keyed by the push event's
  id), a `mirror` when the repository has an active one, and a `repack` past `repack.packs` live packs; a mirror or
  repack already `ready` for the repository covers the push.
- `repack` merges every live pack into one (`packfile.Merge`, oldest first), `Stage`s it, `Commit`s it with no refs,
  then `Retire`s the packs it merged. A rerun after a crash writes the same pack (already live) and retires the rest.
- `prune` (queued nightly by `workers.SchedulePrune`, payload `retention_secs` optional) keeps what refs and open
  changes reach (tit's rules: a change a ref names, one with an open review, or one with a reachable version) plus
  every object of a pack live for less than the retention (default 30 days; tinhub has no oplog), rewrites the packs
  that hold anything else and retires them.
- `purge` (queued by `workers.RequestPurge`, which writes `repo.purge.requested` to `audit_log` in the same
  transaction) refuses an object something reaches (`repo.purge.refused`, the job done), else rewrites the live packs
  without it and deletes at once every retired pack that holds it (`repo.purge`).
- `mirror` writes branches and tags with tit mirror's encoding into `<packs.dir>/mirror/<repo>/mirror.git` (a cache,
  with its git-ids table) and pushes them over smart HTTP; refs deleted in tinhub stay on the mirror.

---

## 7. The pack store (#1011, #1015)

```tin
// package packs: where a repository's packs and capsules are kept
shape PackStore {
	mut Put(key str, path str) !i64         // store the file at path at key, taking the file; the bytes stored
	mut Get(key str, path str) !i64         // the whole object at key into the file at path; the bytes written
	mut ReadAt(key str, off i64, buf mut []u8) !i64   // a ranged read; the bytes read
	mut Size(key str) !?i64                 // nil when there is no object at key
	mut Delete(key str) !bool               // false when it was not there
	mut List(prefix str) ![]str             // every key under prefix, in order
	Local(key str) ?str                     // a file path that holds key's bytes, when the store has one
}
```

(`Put` and `Get` return the bytes rather than a bare `!`: a shape method returning only `!` must be the shape's last,
#1072.) `packs.Check` is the contract every store passes; the tests run it on the directory store and, against
`bench/ref/s3sig`'s signature-checking fake, on the S3 store.

Keys: `repos/<repo id>/packs/<hash>.pack`, `repos/<repo id>/packs/<hash>.idx`, `repos/<repo id>/capsules/<id>.tcap`,
`repos/<repo id>/diffs/<change>/<version>.<against>`. The directory store keeps a key at `<packs.dir>/<key>`, written
through a temporary file in the same directory, synced and renamed. The S3 store keeps it at the same key in the
bucket (multipart above 64 MiB), with the node's copies under `<packs.dir>/cache`: what a node wrote or read stays
there, and the sweep keeps the copies under `packs.cache` by retiring the oldest as tit retires a pack
(`packfile.Retire`: a fetch that opened one follows it into `retired/`) and deleting retired copies after
`packs.grace`. The state of each pack is the `packs` row, never the file: a reader lists live packs from Postgres.

A push over `PgRepo` (`products/tinhub/repo`; design/tit.md §16): the pack is streamed to `TempDir()`
(`<packs.dir>/tmp`), verified and indexed there; `Stage` inserts the `pending` row under the repository's lock (a pack
already live is not stored again; a retired one becomes pending), then `Put`s the `.idx` and the `.pack`; `Commit`
takes the lock (`pg_advisory_xact_lock(-<repo id>)`), re-checks every replaced version and old ref target, sets the
pack `live` (adding its size to the repository's), moves each ref by compare-and-swap (`UPDATE … WHERE target = $old`,
an insert when old is empty, a delete when new is empty, `RefChanged` when no row matched), adds the version rows and
the `push` event, and commits. `Retire` sets `retired_at`. The sweep (every `packs.sweep`, and `tinhub packs sweep`)
deletes, each under its repository's lock, the files and then the rows of `pending` packs older than `push.deadline`
plus 10 minutes and of `retired` packs older than `packs.grace`, and the files a dead push left in `<packs.dir>/tmp`.

Refs are cached in Redis (`redis.addr`) under a generation each commit bumps after it lands (`tinhub:refgen:<repo>`,
`tinhub:refs:<repo>:<gen>`): a reader never finds an entry older than a push that has answered, and a Redis fault or
miss reads Postgres. Pushes never read the cache.

---

## 8. Budgets

| work | `within` | `limit memory` | where |
|---|---|---|---|
| a push | `push.deadline` (60s) | `push.memory` (512mb) | node |
| a fetch or clone | 10m | 256mb | node |
| an API read | 2s | 16mb | node |
| an API write | 5s | 16mb | node |
| a job | its kind's (§6) | its kind's (§6) | worker |
| a replay | the runner's sandbox: memory.max, cpu.max, pids.max, a hard timeout | | runner |

---

## 9. Deployment phases

What a phase changes is configuration and where each store runs; the program is the same. Linux arm64 and x86-64
only.

| | phase 1 (#1024) | phase 2 (#1026) | phase 3 (later) |
|---|---|---|---|
| nodes | one Linux server, every role | several stateless nodes behind a load balancer (`/readyz`); workers as their own nodes | nodes per region |
| Postgres | on the same host | a primary and a replica; refs always on the primary | primary plus regional replicas |
| packs | `packs.store = dir` on local disk | `packs.store = s3`, a pack cache on each node's NVMe | replicated object storage |
| TLS | anvil (TLS 1.3, HTTP/2), certificates from files, SIGHUP reload | the load balancer or anvil | |
| backup | nightly `pg_dump` and an incremental copy of new packs | the provider's snapshots plus the same | |
| move | | config, then `tinhub packs copy` | |

**Not in this milestone:** website pages, public CI, public replay (code from anyone), regions.

---

## 10. Gaps in Tin this design relies on

| need | state | where |
|---|---|---|
| S3 client | `packages/s3` | #1001 |
| a process sandbox | `packages/sandbox` | #1002 |
| `LISTEN/NOTIFY` in `postgres` | missing: workers poll; live updates fan out through Redis or polling until it lands | #1013, #1023 |
| a connection cap that holds (`MaxTotal`) | #856 | phase 1 runs with headroom below Postgres's `max_connections` |
| memory allocated in main survives `anvil.Serve` | #1056 | config is read into `shared let` before cores start |

---

## 11. Accounts, sign-in and sessions (#1012)

`products/tinhub/accounts` implements §5's access rule and design/tit.md §18 on the tables of migration `0001` (no
schema change). The interface the protocol (#1018) and the API (#1019) call:

```tin
const RoleNone = 0; const RoleRead = 1; const RoleWrite = 2; const RoleAdmin = 3
type Repo struct { Id i64; OwnerId i64; Owner str; Name str; Visibility str; DefaultBranch str; QuotaBytes i64; SizeBytes i64 }
type User struct { Id i64; OwnerId i64; Name str; Email str; SiteAdmin bool }
fn FindRepo(c postgres.Client, owner str, name str) !?Repo      // case-insensitive name; deleted repos are nil
fn UserByEmail(c postgres.Client, email str) !?User
fn KeysOf(c postgres.Client, userId i64) ![][]u8                 // raw 32-byte ed25519 public keys
fn RoleOf(c postgres.Client, userId i64, r Repo) !i64           // userId 0 = anonymous
fn Check(c postgres.Client, userId i64, r Repo, need i64) !     // ErrNotFound below read, ErrForbidden below need
fn VerifySigned(c, nonceKey, method, path, signature, bodyHash, now) !User   // a Tit-Signature, by a live key
```

What each role allows: **read** heads, fetch, clone, the API's reads and replay capsules; **write** push; **admin**
rename, visibility, soft delete and grants. A user who cannot read a repository gets `ErrNotFound` (404), never 403.
Org members get nothing from membership alone; org owners are admin of the org's repositories. A deleted user, or a
removed (revoked) key, has no access. Every change that alters access (accounts, site admin, orgs, members, teams,
repositories, grants, keys, invites, approvals and sign-ins) writes its `audit_log` row in its own transaction, with
the actor and the request's address; an invite's code is never logged.

**Invites.** `CreateInvite(c, email, siteAdmin) !str` is what `tinhub admin invite` calls; the code lives 7 days and is
spent once (its row locked `FOR UPDATE`). `POST /tit/v1/keys` (`KeyAdd`) checks that the request is signed by the key it
adds, over the request's own path, then `AcceptInvite` makes the account when the email has none (named after the
email's local part, numbered when taken), adds the key and the invite's org membership and site admin flag.

**Sign-in.** A login request's code lives 10 minutes in `login_requests`: `pending`, then `approved` once by a request
signed with a registered key (a second or late approval is `Moved`), then `used` once when the browser claims it. tit
§18 adds one thing for browsers: `POST /tit/v1/login` also hands the asking browser a claim token (cookie
`tinhub_login`, the code's HMAC under `secrets.cookie`), and only the holder of that token turns the approved code into
a session, so a code seen on someone's screen is worth nothing. The JSON answers are tit's (`LoginAnswer`,
`LoginState`; a used code reads as `approved`); refusals are tit error frames, with a fresh nonce on `Unauthorized`.
`accounts.Service.Handle` answers these routes, at the root or under a repository's URL; the node mounts it.

**Sessions.** A session is the Redis key `tinhub:session:<hex SHA-256(id)>` = `v1 <user id>`, 30 days from sign-in;
the cookie `tinhub_session` is `<id>.<base64url HMAC-SHA256(secrets.cookie, "tinhub session " + id)>`. A node checks the
HMAC before asking Redis, and Redis holds no usable cookie. Every node with the same Redis and `secrets.cookie` reads
every session, so sessions survive restarts. **Losing Redis (or flushing it, or changing `secrets.cookie`) signs
everyone out**; browsers sign in again with `tit login`, and nothing else is lost. A Redis fault on lookup counts as
signed out. Without `redis.addr` there are no browser sessions (`ErrNoSessions`), which replaces §3's "per process"
for sessions: a per-core store would sign a browser in on one core only. Requests signed with `Tit-Signature` need no
session.
