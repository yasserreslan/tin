-- The runner (design/tinhub.md §12, #1028): replays of a repository's sampled capsules against a change version's build,
-- for owners that opted in by making the runner a capsule recipient. Only outcomes are stored: no request, response
-- body or effect key ever leaves the runner's sandbox. Forward only.

-- runner_keys: the runners' public keys, as each registers its own at start; the newest is the one owners opt in to
CREATE TABLE runner_keys (
	key_id text PRIMARY KEY CHECK (key_id ~ '^[0-9a-f]{64}$'),
	public_key text NOT NULL,
	seen_at timestamptz NOT NULL DEFAULT now()
);

-- runner_optins: an owner (an org) whose recording servers seal capsules for the runner key key_id; signers is a comma
-- list of the ed25519 public keys (hex) the runner trusts for its capsules, '' for the signer each capsule names
CREATE TABLE runner_optins (
	owner_id bigint PRIMARY KEY REFERENCES owners (id) ON DELETE CASCADE,
	key_id text NOT NULL,
	signers text NOT NULL DEFAULT '',
	enabled_by bigint REFERENCES users (id) ON DELETE SET NULL,
	created_at timestamptz NOT NULL DEFAULT now()
);

-- runner_settings: what a repository's runs build and replay; no row is entry main.tin, 20 capsules, no environment
CREATE TABLE runner_settings (
	repo_id bigint PRIMARY KEY REFERENCES repos (id) ON DELETE CASCADE,
	entry text NOT NULL DEFAULT 'main.tin',
	sample int NOT NULL DEFAULT 20 CHECK (sample BETWEEN 1 AND 200),
	env text NOT NULL DEFAULT '',
	updated_at timestamptz NOT NULL DEFAULT now()
);

-- runner_runs: one run per change version, run again on request
CREATE TABLE runner_runs (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	change_id text NOT NULL,
	version int NOT NULL CHECK (version >= 1),
	commit_id text NOT NULL,
	base_commit text NOT NULL DEFAULT '',
	state text NOT NULL DEFAULT 'queued' CHECK (state IN ('queued', 'running', 'done', 'failed', 'skipped')),
	reason text NOT NULL DEFAULT '',
	requested_by bigint REFERENCES users (id) ON DELETE SET NULL,
	capsules int NOT NULL DEFAULT 0,
	created_at timestamptz NOT NULL DEFAULT now(),
	started_at timestamptz,
	finished_at timestamptz,
	UNIQUE (repo_id, change_id, version)
);

-- runner_results: each replayed capsule's outcome against the change's build, compared with its base's
CREATE TABLE runner_results (
	run_id bigint NOT NULL REFERENCES runner_runs (id) ON DELETE CASCADE,
	capsule_id text NOT NULL,
	outcome text NOT NULL CHECK (outcome IN ('same', 'body', 'calls', 'error', 'skipped')),
	group_key text NOT NULL,
	label text NOT NULL,
	effect int NOT NULL DEFAULT -1,
	recorded_status int NOT NULL DEFAULT 0,
	base_status int NOT NULL DEFAULT 0,
	change_status int NOT NULL DEFAULT 0,
	created_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (run_id, capsule_id)
);
CREATE INDEX runner_results_group ON runner_results (run_id, outcome, group_key);
