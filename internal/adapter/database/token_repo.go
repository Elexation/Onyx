package database

import (
	"database/sql"
	"fmt"

	"github.com/Elexation/onyx/internal/domain"
)

type TokenRepo struct {
	db *sql.DB
}

func NewTokenRepo(db *sql.DB) *TokenRepo {
	return &TokenRepo{db: db}
}

func (r *TokenRepo) Create(name, tokenHash, tokenLast8, scope string, createdAt int64, expiresAt *int64) (int64, error) {
	res, err := r.db.Exec(
		"INSERT INTO personal_access_tokens (name, token_hash, token_last8, scope, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)",
		name, tokenHash, tokenLast8, scope, createdAt, expiresAt,
	)
	if err != nil {
		return 0, fmt.Errorf("insert token: %w", err)
	}
	return res.LastInsertId()
}

func (r *TokenRepo) GetByTokenHash(tokenHash string) (*domain.PersonalAccessToken, error) {
	var tok domain.PersonalAccessToken
	var lastUsed, expiresAt sql.NullInt64
	err := r.db.QueryRow(
		"SELECT id, name, token_last8, scope, created_at, last_used_at, expires_at FROM personal_access_tokens WHERE token_hash = ?",
		tokenHash,
	).Scan(&tok.ID, &tok.Name, &tok.TokenLast8, &tok.Scope, &tok.CreatedAt, &lastUsed, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get token by hash: %w", err)
	}
	if lastUsed.Valid {
		tok.LastUsedAt = lastUsed.Int64
	}
	if expiresAt.Valid {
		tok.ExpiresAt = expiresAt.Int64
	}
	return &tok, nil
}

func (r *TokenRepo) List() ([]domain.PersonalAccessToken, error) {
	rows, err := r.db.Query(
		"SELECT id, name, token_last8, scope, created_at, last_used_at, expires_at FROM personal_access_tokens ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}
	defer rows.Close()

	var tokens []domain.PersonalAccessToken
	for rows.Next() {
		var tok domain.PersonalAccessToken
		var lastUsed, expiresAt sql.NullInt64
		if err := rows.Scan(&tok.ID, &tok.Name, &tok.TokenLast8, &tok.Scope, &tok.CreatedAt, &lastUsed, &expiresAt); err != nil {
			return nil, fmt.Errorf("scan token: %w", err)
		}
		if lastUsed.Valid {
			tok.LastUsedAt = lastUsed.Int64
		}
		if expiresAt.Valid {
			tok.ExpiresAt = expiresAt.Int64
		}
		tokens = append(tokens, tok)
	}
	return tokens, rows.Err()
}

func (r *TokenRepo) Delete(id int64) error {
	res, err := r.db.Exec("DELETE FROM personal_access_tokens WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete token: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("token not found")
	}
	return nil
}

func (r *TokenRepo) Count() (int64, error) {
	var n int64
	err := r.db.QueryRow("SELECT COUNT(*) FROM personal_access_tokens").Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count tokens: %w", err)
	}
	return n, nil
}

func (r *TokenRepo) UpdateLastUsed(id, ts int64) error {
	_, err := r.db.Exec("UPDATE personal_access_tokens SET last_used_at = ? WHERE id = ?", ts, id)
	if err != nil {
		return fmt.Errorf("update last_used_at: %w", err)
	}
	return nil
}

func (r *TokenRepo) DeleteExpired(now int64) (int64, error) {
	res, err := r.db.Exec("DELETE FROM personal_access_tokens WHERE expires_at IS NOT NULL AND expires_at < ?", now)
	if err != nil {
		return 0, fmt.Errorf("delete expired tokens: %w", err)
	}
	return res.RowsAffected()
}
