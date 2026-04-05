package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Elexation/onyx/internal/domain"
)

type SessionRepo struct {
	db *sql.DB
}

func NewSessionRepo(db *sql.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

func (r *SessionRepo) Create(s *domain.Session) error {
	_, err := r.db.Exec(
		"INSERT INTO sessions (id, user_id, csrf_token, created_at, last_active_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)",
		s.ID, s.UserID, s.CSRFToken, s.CreatedAt, s.LastActiveAt, s.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *SessionRepo) GetByID(id string) (*domain.Session, error) {
	s := &domain.Session{}
	err := r.db.QueryRow(
		"SELECT id, user_id, csrf_token, created_at, last_active_at, expires_at FROM sessions WHERE id = ?",
		id,
	).Scan(&s.ID, &s.UserID, &s.CSRFToken, &s.CreatedAt, &s.LastActiveAt, &s.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return s, nil
}

func (r *SessionRepo) UpdateLastActive(id string) error {
	_, err := r.db.Exec(
		"UPDATE sessions SET last_active_at = ? WHERE id = ?",
		time.Now().Unix(), id,
	)
	if err != nil {
		return fmt.Errorf("update session last active: %w", err)
	}
	return nil
}

func (r *SessionRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM sessions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (r *SessionRepo) DeleteExpired() (int64, error) {
	result, err := r.db.Exec("DELETE FROM sessions WHERE expires_at < ?", time.Now().Unix())
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return result.RowsAffected()
}
