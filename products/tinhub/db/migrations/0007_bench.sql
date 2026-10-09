-- Benchmark history (design/tinhub.md §5, §13, #1029): the bench job reads tit bench's result files from pushed trees
-- (.bench/<name>.jsonl) into bench_results. Each row keeps the change its result was recorded for (tit bench follows the
-- change id, so a result survives amends) and the hash of its line, so a file read again (it grows commit by commit,
-- and a job may run twice) adds nothing twice. bench_files remembers the blobs read already. Forward only.

ALTER TABLE bench_results ADD COLUMN change_id text NOT NULL DEFAULT '';
ALTER TABLE bench_results ADD COLUMN line_hash text NOT NULL DEFAULT '';
CREATE UNIQUE INDEX bench_results_line ON bench_results (repo_id, line_hash) WHERE line_hash <> '';
CREATE INDEX bench_results_change ON bench_results (repo_id, change_id);
CREATE INDEX bench_results_commit ON bench_results (repo_id, commit_id);

-- bench_files: a results file (a blob) whose lines are in bench_results already
CREATE TABLE bench_files (
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	blob text NOT NULL CHECK (blob ~ '^[0-9a-f]{64}$'),
	lines int NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (repo_id, blob)
);
