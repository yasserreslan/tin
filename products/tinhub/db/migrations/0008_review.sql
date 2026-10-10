-- Reviews, comments, checks and landing (#1025): each repository's review settings (how many writers' approvals a
-- change needs, and the checks that must pass on it), what landing recorded on a review (the commit that went onto
-- the target, and who landed it), the version a comment was written on (its version column is the one it is anchored
-- to now) and when it went outdated (its declaration renamed or deleted), and a check's one-line description.

CREATE TABLE review_settings (
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	approvals int NOT NULL DEFAULT 1 CHECK (approvals BETWEEN 0 AND 10),
	required_checks text[] NOT NULL DEFAULT '{}',
	updated_by bigint REFERENCES users (id) ON DELETE SET NULL,
	updated_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (repo_id)
);

ALTER TABLE reviews ADD COLUMN landed_commit text NOT NULL DEFAULT '';
ALTER TABLE reviews ADD COLUMN landed_by bigint REFERENCES users (id) ON DELETE SET NULL;

ALTER TABLE comments ADD COLUMN written_on int NOT NULL DEFAULT 0;
ALTER TABLE comments ADD COLUMN outdated_at timestamptz;

ALTER TABLE checks ADD COLUMN description text NOT NULL DEFAULT '';
CREATE INDEX checks_change ON checks (repo_id, change_id);
