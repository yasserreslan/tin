-- The replay store (design/tinhub.md section 5, #1022): the separate replay permission, each repository's retention,
-- and the declaration a failure group links to. Forward only.

-- replay_grants: who may read a repository's capsules, besides its admins; a user or a team, as grants
CREATE TABLE replay_grants (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	user_id bigint REFERENCES users (id) ON DELETE CASCADE,
	team_id bigint REFERENCES teams (id) ON DELETE CASCADE,
	created_at timestamptz NOT NULL DEFAULT now(),
	CHECK ((user_id IS NULL) <> (team_id IS NULL)),
	UNIQUE (repo_id, user_id),
	UNIQUE (repo_id, team_id)
);

-- replay_settings: a repository's capsule retention; no row is the default (30 days)
CREATE TABLE replay_settings (
	repo_id bigint PRIMARY KEY REFERENCES repos (id) ON DELETE CASCADE,
	retention_days int NOT NULL CHECK (retention_days BETWEEN 1 AND 3650),
	updated_at timestamptz NOT NULL DEFAULT now()
);

-- the declaration that panicked (package.name), from symbols or from a replay; "" when not known yet
ALTER TABLE replay_groups ADD COLUMN decl text NOT NULL DEFAULT '';

CREATE INDEX capsules_newest ON capsules (repo_id, created_at);
CREATE INDEX capsules_group ON capsules (repo_id, group_id, created_at);
CREATE INDEX replay_groups_fixed ON replay_groups (repo_id, fixed_by) WHERE fixed_by <> '';
