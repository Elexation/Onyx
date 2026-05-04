package domain

type ShareLink struct {
	ID            int64  `json:"id"`
	Token         string `json:"token,omitempty"`
	TokenLast8    string `json:"tokenLast8"`
	FilePath      string `json:"filePath"`
	IsDir         bool   `json:"isDir"`
	CreatedAt     int64  `json:"createdAt"`
	ExpiresAt     int64  `json:"expiresAt,omitempty"`
	HasPassword   bool   `json:"hasPassword"`
	DownloadCount int    `json:"downloadCount"`
}
