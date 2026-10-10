-- The symbol index (#1020) and review diffs and overlaps (#1021): a method's receiver beside its name, the commits
-- the index holds (how each was made), and the declarations each change version touches, which overlaps join on.

ALTER TABLE symbols ADD COLUMN recv text NOT NULL DEFAULT '';
CREATE INDEX symbols_repo_name ON symbols (repo_id, lower(name));

CREATE TABLE indexed_commits (
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	commit_id text NOT NULL,
	packages int NOT NULL,
	reindexed int NOT NULL,
	base text NOT NULL DEFAULT '',
	runner text NOT NULL CHECK (runner IN ('sandbox', 'process')),
	created_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (repo_id, commit_id)
);

CREATE TABLE change_decls (
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	change_id text NOT NULL,
	version int NOT NULL,
	decl text NOT NULL,
	PRIMARY KEY (repo_id, change_id, version, decl)
);
CREATE INDEX change_decls_decl ON change_decls (repo_id, decl);
