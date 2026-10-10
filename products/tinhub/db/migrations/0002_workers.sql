-- Mirrors (#1014): a repository copied to a git host after every push, with tit mirror's encoding (#805).
-- url is https://host/owner/repo.git (or a local git directory, where the worker allows it); token, when not '', is
-- the password of HTTP basic credentials (GitHub: a token). Only a repository's admins set them.
CREATE TABLE mirrors (
	repo_id bigint PRIMARY KEY REFERENCES repos (id) ON DELETE CASCADE,
	url text NOT NULL CHECK (url <> ''),
	token text NOT NULL DEFAULT '',
	active boolean NOT NULL DEFAULT true,
	created_at timestamptz NOT NULL DEFAULT now(),
	mirrored_at timestamptz,
	last_error text NOT NULL DEFAULT ''
);
