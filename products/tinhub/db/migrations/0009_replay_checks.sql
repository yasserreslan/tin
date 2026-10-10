-- Replay checks (design/tinhub.md §12.1): any branch, tag, change or commit of a repository built by the runner and
-- replayed against one capsule or a failure group's capsules, each judged against its recording: passed, diverged
-- (and at which effect), panicked, still failing, timed out or skipped. As for runs, only outcomes are stored: no
-- request, response body or effect key; a panic is kept as the function it happened in, and its message only when it
-- is the failure group's own (already readable with the replay permission).

CREATE TABLE runner_checks (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	-- target is what was asked for (a branch, a tag, a change id or a commit id), commit_id what it named then
	target text NOT NULL,
	commit_id text NOT NULL,
	-- the failure group checked, or one capsule of it (capsule_id set)
	group_id text NOT NULL,
	capsule_id text NOT NULL DEFAULT '',
	state text NOT NULL DEFAULT 'queued' CHECK (state IN ('queued', 'running', 'done', 'failed', 'skipped')),
	reason text NOT NULL DEFAULT '',
	capsules int NOT NULL DEFAULT 0,
	requested_by bigint REFERENCES users (id) ON DELETE SET NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	started_at timestamptz,
	finished_at timestamptz
);
CREATE INDEX runner_checks_group ON runner_checks (repo_id, group_id, created_at DESC);

-- runner_check_results: each capsule's verdict; detail is the explanation the page shows
CREATE TABLE runner_check_results (
	check_id bigint NOT NULL REFERENCES runner_checks (id) ON DELETE CASCADE,
	capsule_id text NOT NULL,
	verdict text NOT NULL CHECK (verdict IN ('passed', 'diverged', 'panicked', 'failing', 'timeout', 'skipped')),
	label text NOT NULL,
	detail text NOT NULL DEFAULT '',
	status int NOT NULL DEFAULT 0,
	recorded_status int NOT NULL DEFAULT 0,
	effect int NOT NULL DEFAULT -1,
	got text NOT NULL DEFAULT '',
	want text NOT NULL DEFAULT '',
	left_over int NOT NULL DEFAULT 0,
	panic_in text NOT NULL DEFAULT '',
	PRIMARY KEY (check_id, capsule_id)
);
