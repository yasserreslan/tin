-- tinhub's first schema (design/tinhub.md section 5). Forward only: later changes are new numbered files.

CREATE TABLE owners (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	name text NOT NULL UNIQUE CHECK (name ~ '^[a-z0-9][a-z0-9-]{0,38}$'),
	kind text NOT NULL CHECK (kind IN ('user', 'org')),
	created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	owner_id bigint NOT NULL UNIQUE REFERENCES owners (id) ON DELETE CASCADE,
	email text NOT NULL,
	display text NOT NULL DEFAULT '',
	site_admin boolean NOT NULL DEFAULT false,
	created_at timestamptz NOT NULL DEFAULT now(),
	deleted_at timestamptz
);
CREATE UNIQUE INDEX users_email ON users (lower(email));

CREATE TABLE orgs (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	owner_id bigint NOT NULL UNIQUE REFERENCES owners (id) ON DELETE CASCADE,
	display text NOT NULL DEFAULT '',
	created_at timestamptz NOT NULL DEFAULT now(),
	deleted_at timestamptz
);

CREATE TABLE members (
	org_id bigint NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
	user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	role text NOT NULL CHECK (role IN ('member', 'owner')),
	created_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (org_id, user_id)
);

CREATE TABLE teams (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	org_id bigint NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
	name text NOT NULL CHECK (name ~ '^[a-z0-9][a-z0-9-]{0,38}$'),
	created_at timestamptz NOT NULL DEFAULT now(),
	UNIQUE (org_id, name)
);

CREATE TABLE team_members (
	team_id bigint NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
	user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	PRIMARY KEY (team_id, user_id)
);

CREATE TABLE repos (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	owner_id bigint NOT NULL REFERENCES owners (id) ON DELETE CASCADE,
	name text NOT NULL CHECK (name ~ '^[A-Za-z0-9._-]{1,100}$' AND name NOT IN ('.', '..') AND name NOT LIKE '%.tit'),
	visibility text NOT NULL CHECK (visibility IN ('public', 'private')),
	description text NOT NULL DEFAULT '',
	default_branch text NOT NULL DEFAULT 'main',
	quota_bytes bigint NOT NULL CHECK (quota_bytes >= 0),
	size_bytes bigint NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
	push_seq bigint NOT NULL DEFAULT 0,
	created_at timestamptz NOT NULL DEFAULT now(),
	deleted_at timestamptz
);
CREATE UNIQUE INDEX repos_name ON repos (owner_id, lower(name)) WHERE deleted_at IS NULL;

CREATE TABLE grants (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	user_id bigint REFERENCES users (id) ON DELETE CASCADE,
	team_id bigint REFERENCES teams (id) ON DELETE CASCADE,
	role text NOT NULL CHECK (role IN ('read', 'write', 'admin')),
	CHECK ((user_id IS NULL) <> (team_id IS NULL)),
	UNIQUE (repo_id, user_id),
	UNIQUE (repo_id, team_id)
);

CREATE TABLE keys (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	public_key text NOT NULL UNIQUE CHECK (length(public_key) = 44),
	name text NOT NULL DEFAULT '',
	created_at timestamptz NOT NULL DEFAULT now(),
	last_used_at timestamptz,
	revoked_at timestamptz
);
CREATE INDEX keys_user ON keys (user_id);

CREATE TABLE invites (
	code text PRIMARY KEY,
	email text NOT NULL,
	org_id bigint REFERENCES orgs (id) ON DELETE CASCADE,
	site_admin boolean NOT NULL DEFAULT false,
	created_by bigint REFERENCES users (id) ON DELETE SET NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	expires_at timestamptz NOT NULL,
	used_at timestamptz,
	used_by bigint REFERENCES users (id) ON DELETE SET NULL
);

CREATE TABLE login_requests (
	code text PRIMARY KEY,
	state text NOT NULL CHECK (state IN ('pending', 'approved', 'used')),
	user_id bigint REFERENCES users (id) ON DELETE CASCADE,
	created_at timestamptz NOT NULL DEFAULT now(),
	expires_at timestamptz NOT NULL,
	approved_at timestamptz,
	CHECK ((state = 'pending') = (user_id IS NULL))
);

CREATE TABLE audit_log (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	at timestamptz NOT NULL DEFAULT now(),
	actor_id bigint REFERENCES users (id) ON DELETE SET NULL,
	action text NOT NULL,
	target_kind text NOT NULL,
	target_id bigint NOT NULL,
	detail jsonb NOT NULL DEFAULT '{}',
	ip text NOT NULL DEFAULT ''
);
CREATE INDEX audit_target ON audit_log (target_kind, target_id, at);

CREATE TABLE refs (
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	name text NOT NULL,
	target text NOT NULL CHECK (target <> ''),
	updated_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (repo_id, name)
);

CREATE TABLE changes (
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	change_id text NOT NULL,
	version int NOT NULL CHECK (version >= 1),
	commit_id text NOT NULL CHECK (commit_id ~ '^[0-9a-f]{64}$'),
	op bigint NOT NULL,
	pushed_by bigint REFERENCES users (id) ON DELETE SET NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (repo_id, change_id, version),
	UNIQUE (repo_id, change_id, op)
);

CREATE TABLE packs (
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	hash text NOT NULL CHECK (hash ~ '^[0-9a-f]{64}$'),
	state text NOT NULL CHECK (state IN ('pending', 'live', 'retired')),
	size_bytes bigint NOT NULL DEFAULT 0,
	objects bigint NOT NULL DEFAULT 0,
	staged_at timestamptz NOT NULL DEFAULT now(),
	live_at timestamptz,
	retired_at timestamptz,
	PRIMARY KEY (repo_id, hash),
	CHECK ((state = 'pending' AND live_at IS NULL AND retired_at IS NULL)
		OR (state = 'live' AND live_at IS NOT NULL AND retired_at IS NULL)
		OR (state = 'retired' AND retired_at IS NOT NULL))
);
CREATE INDEX packs_state ON packs (state, staged_at);

CREATE TABLE reviews (
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	change_id text NOT NULL,
	state text NOT NULL CHECK (state IN ('open', 'approved', 'changes_requested', 'landed', 'abandoned')),
	target text NOT NULL,
	opened_by bigint REFERENCES users (id) ON DELETE SET NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	landed_at timestamptz,
	PRIMARY KEY (repo_id, change_id)
);

CREATE TABLE approvals (
	repo_id bigint NOT NULL,
	change_id text NOT NULL,
	version int NOT NULL,
	user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	vote text NOT NULL CHECK (vote IN ('approve', 'changes')),
	created_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (repo_id, change_id, version, user_id),
	FOREIGN KEY (repo_id, change_id) REFERENCES reviews (repo_id, change_id) ON DELETE CASCADE
);

CREATE TABLE comments (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	repo_id bigint NOT NULL,
	change_id text NOT NULL,
	version int NOT NULL,
	author_id bigint REFERENCES users (id) ON DELETE SET NULL,
	parent_id bigint REFERENCES comments (id) ON DELETE CASCADE,
	decl text NOT NULL DEFAULT '',
	file text NOT NULL DEFAULT '',
	line_offset int NOT NULL DEFAULT 0,
	body text NOT NULL,
	outdated boolean NOT NULL DEFAULT false,
	created_at timestamptz NOT NULL DEFAULT now(),
	resolved_at timestamptz,
	FOREIGN KEY (repo_id, change_id) REFERENCES reviews (repo_id, change_id) ON DELETE CASCADE
);
CREATE INDEX comments_change ON comments (repo_id, change_id);

CREATE TABLE checks (
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	change_id text NOT NULL,
	version int NOT NULL,
	name text NOT NULL,
	state text NOT NULL CHECK (state IN ('pending', 'success', 'failure', 'error')),
	url text NOT NULL DEFAULT '',
	key_id bigint REFERENCES keys (id) ON DELETE SET NULL,
	updated_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (repo_id, change_id, version, name)
);

CREATE TABLE symbols (
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	commit_id text NOT NULL,
	package text NOT NULL,
	name text NOT NULL,
	kind text NOT NULL,
	file text NOT NULL,
	line int NOT NULL,
	end_line int NOT NULL
);
CREATE INDEX symbols_commit ON symbols (repo_id, commit_id);
CREATE INDEX symbols_name ON symbols (lower(name));

CREATE TABLE change_overlaps (
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	change_id text NOT NULL,
	version int NOT NULL,
	other_change_id text NOT NULL,
	decl text NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (repo_id, change_id, version, other_change_id, decl)
);

CREATE TABLE diffs (
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	change_id text NOT NULL,
	version int NOT NULL,
	against text NOT NULL CHECK (against IN ('base', 'previous')),
	kind text NOT NULL CHECK (kind IN ('semantic', 'lines')),
	body text NOT NULL DEFAULT '',
	stored boolean NOT NULL DEFAULT false,
	created_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (repo_id, change_id, version, against)
);

CREATE TABLE bench_results (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	commit_id text NOT NULL,
	name text NOT NULL,
	value double precision NOT NULL,
	unit text NOT NULL DEFAULT '',
	os text NOT NULL,
	arch text NOT NULL,
	cpu text NOT NULL DEFAULT '',
	kernel text NOT NULL DEFAULT '',
	machine text NOT NULL DEFAULT '',
	created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX bench_series ON bench_results (repo_id, name, created_at);

CREATE TABLE replay_groups (
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	id text NOT NULL,
	panic text NOT NULL DEFAULT '',
	route text NOT NULL DEFAULT '',
	state text NOT NULL CHECK (state IN ('open', 'closed')),
	fixed_by text NOT NULL DEFAULT '',
	reopened_by text NOT NULL DEFAULT '',
	count bigint NOT NULL DEFAULT 0,
	first_at timestamptz NOT NULL DEFAULT now(),
	last_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (repo_id, id)
);

CREATE TABLE capsules (
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	id text NOT NULL CHECK (id ~ '^[0-9a-f]{64}$'),
	group_id text NOT NULL,
	commit_id text NOT NULL DEFAULT '',
	name text NOT NULL DEFAULT '',
	signer text NOT NULL DEFAULT '',
	summary jsonb NOT NULL DEFAULT '{}',
	status int NOT NULL DEFAULT 0,
	route text NOT NULL DEFAULT '',
	size_bytes bigint NOT NULL DEFAULT 0,
	created_at timestamptz NOT NULL DEFAULT now(),
	expires_at timestamptz NOT NULL,
	PRIMARY KEY (repo_id, id),
	FOREIGN KEY (repo_id, group_id) REFERENCES replay_groups (repo_id, id) ON DELETE CASCADE
);
CREATE INDEX capsules_expiry ON capsules (expires_at);

CREATE TABLE events (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	kind text NOT NULL,
	repo_id bigint REFERENCES repos (id) ON DELETE CASCADE,
	payload jsonb NOT NULL DEFAULT '{}',
	state text NOT NULL DEFAULT 'ready' CHECK (state IN ('ready', 'claimed', 'done', 'dead')),
	attempts int NOT NULL DEFAULT 0,
	next_at timestamptz NOT NULL DEFAULT now(),
	lease_until timestamptz,
	claimed_by text NOT NULL DEFAULT '',
	last_error text NOT NULL DEFAULT '',
	created_at timestamptz NOT NULL DEFAULT now(),
	done_at timestamptz,
	CHECK ((state = 'claimed') = (lease_until IS NOT NULL))
);
CREATE INDEX events_ready ON events (next_at) WHERE state = 'ready';
CREATE INDEX events_claimed ON events (lease_until) WHERE state = 'claimed';

CREATE TABLE webhooks (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	url text NOT NULL,
	secret text NOT NULL,
	kinds text[] NOT NULL DEFAULT '{}',
	active boolean NOT NULL DEFAULT true,
	created_by bigint REFERENCES users (id) ON DELETE SET NULL,
	created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE deliveries (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	webhook_id bigint NOT NULL REFERENCES webhooks (id) ON DELETE CASCADE,
	event_id bigint NOT NULL REFERENCES events (id) ON DELETE CASCADE,
	attempt int NOT NULL,
	status int NOT NULL DEFAULT 0,
	error text NOT NULL DEFAULT '',
	duration_ms int NOT NULL DEFAULT 0,
	created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX deliveries_webhook ON deliveries (webhook_id, created_at);
