package domain

type FileVersion struct {
	ID          int64  `json:"id"`
	FilePath    string `json:"filePath"`
	VersionPath string `json:"-"`
	CreatedAt   int64  `json:"createdAt"`
	Size        int64  `json:"size"`
}
