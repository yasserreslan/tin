-- Notifications (#1023): who follows a repository or one of its changes, and which notifications were mailed, so a
-- notify job that runs again (delivery is at least once) mails nobody twice.

CREATE TABLE subscriptions (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	repo_id bigint NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
	change_id text NOT NULL DEFAULT '',
	created_at timestamptz NOT NULL DEFAULT now(),
	UNIQUE (user_id, repo_id, change_id)
);
CREATE INDEX subscriptions_repo ON subscriptions (repo_id, change_id);

CREATE TABLE notifications (
	event_id bigint NOT NULL REFERENCES events (id) ON DELETE CASCADE,
	user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	sent_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (event_id, user_id)
);
