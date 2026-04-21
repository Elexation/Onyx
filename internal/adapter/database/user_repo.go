package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Elexation/onyx/internal/domain"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(username, passwordHash string) (*domain.User, error) {
	now := time.Now().Unix()
	result, err := r.db.Exec(
		"INSERT INTO users (username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?)",
		username, passwordHash, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	id, _ := result.LastInsertId()
	return &domain.User{
		ID:           id,
		Username:     username,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (r *UserRepo) GetByUsername(username string) (*domain.User, error) {
	u := &domain.User{}
	err := r.db.QueryRow(
		"SELECT id, username, password_hash, created_at, updated_at FROM users WHERE username = ?",
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return u, nil
}

func (r *UserRepo) UpdatePassword(username, passwordHash string) error {
	_, err := r.db.Exec(
		"UPDATE users SET password_hash = ?, updated_at = ? WHERE username = ?",
		passwordHash, time.Now().Unix(), username,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

func (r *UserRepo) Exists() (bool, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check user exists: %w", err)
	}
	return count > 0, nil
}
