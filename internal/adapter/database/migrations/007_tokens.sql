CREATE TABLE personal_access_tokens (
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	token_hash TEXT NOT NULL UNIQUE,
	token_last8 TEXT NOT NULL,
	scope TEXT NOT NULL DEFAULT 'full',
	created_at INTEGER NOT NULL,
	last_used_at INTEGER,
	expires_at INTEGER
);
CREATE INDEX idx_tokens_hash ON personal_access_tokens(token_hash);
CREATE INDEX idx_tokens_expires ON personal_access_tokens(expires_at);
