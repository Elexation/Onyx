package domain

type Session struct {
	ID           string
	UserID       int64
	CSRFToken    string
	CreatedAt    int64
	LastActiveAt int64
	ExpiresAt    int64
}
