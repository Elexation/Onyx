CREATE TABLE file_versions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	file_path TEXT NOT NULL,
	version_path TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	size INTEGER NOT NULL
);
CREATE INDEX idx_versions_file ON file_versions(file_path, created_at DESC);
