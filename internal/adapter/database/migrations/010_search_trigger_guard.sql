-- token_hash is UNIQUE in both tables, so SQLite already keeps an index for it.
DROP INDEX IF EXISTS idx_shares_token;
DROP INDEX IF EXISTS idx_tokens_hash;

-- Rescans upsert every row; only reindex FTS when name or path actually changed.
-- Must stay AFTER: a BEFORE trigger reads the wrong row state and corrupts the index.
DROP TRIGGER IF EXISTS files_au;
CREATE TRIGGER files_au AFTER UPDATE ON files
WHEN old.name IS NOT new.name OR old.path IS NOT new.path
BEGIN
	INSERT INTO file_search(file_search, rowid, name, path) VALUES('delete', old.id, old.name, old.path);
	INSERT INTO file_search(rowid, name, path) VALUES (new.id, new.name, new.path);
END;
