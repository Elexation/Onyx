package database

import (
	"database/sql"
	"fmt"

	"github.com/Elexation/onyx/internal/domain"
)

type ShareRepo struct {
	db *sql.DB
}

func NewShareRepo(db *sql.DB) *ShareRepo {
	return &ShareRepo{db: db}
}

func (r *ShareRepo) Create(tokenHash, tokenLast8, filePath string, isDir bool, createdAt int64, expiresAt *int64, passwordHash *string) (int64, error) {
	res, err := r.db.Exec(
		"INSERT INTO share_links (token_hash, token_last8, file_path, is_dir, created_at, expires_at, password_hash) VALUES (?, ?, ?, ?, ?, ?, ?)",
		tokenHash, tokenLast8, filePath, isDir, createdAt, expiresAt, passwordHash,
	)
	if err != nil {
		return 0, fmt.Errorf("insert share link: %w", err)
	}
	return res.LastInsertId()
}

func (r *ShareRepo) GetByTokenHash(tokenHash string) (*domain.ShareLink, *string, error) {
	var link domain.ShareLink
	var isDir int
	var expiresAt sql.NullInt64
	var passwordHash sql.NullString
	err := r.db.QueryRow(
		"SELECT id, token_last8, file_path, is_dir, created_at, expires_at, password_hash, download_count FROM share_links WHERE token_hash = ?",
		tokenHash,
	).Scan(&link.ID, &link.TokenLast8, &link.FilePath, &isDir, &link.CreatedAt, &expiresAt, &passwordHash, &link.DownloadCount)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get share by token: %w", err)
	}
	link.IsDir = isDir != 0
	if expiresAt.Valid {
		link.ExpiresAt = expiresAt.Int64
	}
	link.HasPassword = passwordHash.Valid
	var pwHash *string
	if passwordHash.Valid {
		pwHash = &passwordHash.String
	}
	return &link, pwHash, nil
}

func (r *ShareRepo) GetByPath(filePath string) (*domain.ShareLink, error) {
	var link domain.ShareLink
	var isDir int
	var expiresAt sql.NullInt64
	var passwordHash sql.NullString
	err := r.db.QueryRow(
		"SELECT id, token_last8, file_path, is_dir, created_at, expires_at, password_hash, download_count FROM share_links WHERE file_path = ?",
		filePath,
	).Scan(&link.ID, &link.TokenLast8, &link.FilePath, &isDir, &link.CreatedAt, &expiresAt, &passwordHash, &link.DownloadCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get share by path: %w", err)
	}
	link.IsDir = isDir != 0
	if expiresAt.Valid {
		link.ExpiresAt = expiresAt.Int64
	}
	link.HasPassword = passwordHash.Valid
	return &link, nil
}

func (r *ShareRepo) List() ([]domain.ShareLink, error) {
	rows, err := r.db.Query(
		"SELECT id, token_last8, file_path, is_dir, created_at, expires_at, password_hash, download_count FROM share_links ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("list shares: %w", err)
	}
	defer rows.Close()

	var links []domain.ShareLink
	for rows.Next() {
		var link domain.ShareLink
		var isDir int
		var expiresAt sql.NullInt64
		var passwordHash sql.NullString
		if err := rows.Scan(&link.ID, &link.TokenLast8, &link.FilePath, &isDir, &link.CreatedAt, &expiresAt, &passwordHash, &link.DownloadCount); err != nil {
			return nil, fmt.Errorf("scan share link: %w", err)
		}
		link.IsDir = isDir != 0
		if expiresAt.Valid {
			link.ExpiresAt = expiresAt.Int64
		}
		link.HasPassword = passwordHash.Valid
		links = append(links, link)
	}
	return links, rows.Err()
}

func (r *ShareRepo) Delete(id int64) error {
	res, err := r.db.Exec("DELETE FROM share_links WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete share link: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("share link not found")
	}
	return nil
}

func (r *ShareRepo) IncrementDownloadCount(id int64) error {
	_, err := r.db.Exec("UPDATE share_links SET download_count = download_count + 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("increment download count: %w", err)
	}
	return nil
}

func (r *ShareRepo) DeleteAll() (int64, error) {
	res, err := r.db.Exec("DELETE FROM share_links")
	if err != nil {
		return 0, fmt.Errorf("delete all shares: %w", err)
	}
	return res.RowsAffected()
}

func (r *ShareRepo) Count() (int64, error) {
	var n int64
	err := r.db.QueryRow("SELECT COUNT(*) FROM share_links").Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count shares: %w", err)
	}
	return n, nil
}

func (r *ShareRepo) DeleteExpired(now int64) (int64, error) {
	res, err := r.db.Exec("DELETE FROM share_links WHERE expires_at IS NOT NULL AND expires_at < ?", now)
	if err != nil {
		return 0, fmt.Errorf("delete expired shares: %w", err)
	}
	return res.RowsAffected()
}
