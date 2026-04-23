package domain

type TrashItem struct {
	ID           string `json:"id"`
	OriginalPath string `json:"originalPath"`
	TrashPath    string `json:"-"`
	DeletedAt    int64  `json:"deletedAt"`
	Size         int64  `json:"size"`
	IsDir        bool   `json:"isDir"`
}
