CREATE TABLE share_links (
	id INTEGER PRIMARY KEY,
	token_hash TEXT NOT NULL UNIQUE,
	token_last8 TEXT NOT NULL,
	file_path TEXT NOT NULL,
	is_dir INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL,
	expires_at INTEGER,
	password_hash TEXT,
	download_count INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_shares_token ON share_links(token_hash);
CREATE INDEX idx_shares_expires ON share_links(expires_at);
