package domain

const (
	ScopeRead   = "read"
	ScopeUpload = "upload"
	ScopeFull   = "full"
)

// IsValidTokenScope reports whether the given scope is one of the recognized
// PAT scopes.
func IsValidTokenScope(s string) bool {
	return s == ScopeRead || s == ScopeUpload || s == ScopeFull
}

type PersonalAccessToken struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Token      string `json:"token,omitempty"`
	TokenLast8 string `json:"tokenLast8"`
	Scope      string `json:"scope"`
	CreatedAt  int64  `json:"createdAt"`
	LastUsedAt int64  `json:"lastUsedAt,omitempty"`
	ExpiresAt  int64  `json:"expiresAt,omitempty"`
}
