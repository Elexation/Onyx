-- Remove duplicates before adding unique constraint: keep the newest share per file_path
DELETE FROM share_links WHERE id NOT IN (
	SELECT MAX(id) FROM share_links GROUP BY file_path
);
CREATE UNIQUE INDEX idx_shares_file_path ON share_links(file_path);
