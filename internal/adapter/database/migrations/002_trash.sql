CREATE TABLE trash_items (
	id TEXT PRIMARY KEY,
	original_path TEXT NOT NULL,
	trash_path TEXT NOT NULL,
	deleted_at INTEGER NOT NULL,
	size INTEGER NOT NULL,
	is_dir INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_trash_deleted ON trash_items(deleted_at);
