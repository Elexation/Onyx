CREATE TABLE files (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	path TEXT NOT NULL UNIQUE,
	is_dir INTEGER NOT NULL DEFAULT 0,
	size INTEGER NOT NULL DEFAULT 0,
	mod_time INTEGER NOT NULL DEFAULT 0,
	indexed_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_files_path ON files(path);

CREATE VIRTUAL TABLE file_search USING fts5(
	name,
	path,
	content='files',
	content_rowid='id',
	tokenize='unicode61'
);

CREATE TRIGGER files_ai AFTER INSERT ON files BEGIN
	INSERT INTO file_search(rowid, name, path) VALUES (new.id, new.name, new.path);
END;

CREATE TRIGGER files_ad AFTER DELETE ON files BEGIN
	INSERT INTO file_search(file_search, rowid, name, path) VALUES('delete', old.id, old.name, old.path);
END;

CREATE TRIGGER files_au AFTER UPDATE ON files BEGIN
	INSERT INTO file_search(file_search, rowid, name, path) VALUES('delete', old.id, old.name, old.path);
	INSERT INTO file_search(rowid, name, path) VALUES (new.id, new.name, new.path);
END;
