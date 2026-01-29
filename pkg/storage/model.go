package storage

// UploadResult represents the result of a successful object upload
type UploadResult struct {
	Key       string `json:"key"`
	Region    string `json:"region,omitempty"`
	SizeBytes int64  `json:"sizeBytes"`
	LocalPath string `json:"localPath,omitempty"`
}

// DownloadResult represents the result of a successful object download
type DownloadResult struct {
	Key       string `json:"key"`
	Region    string `json:"region,omitempty"`
	SizeBytes int64  `json:"sizeBytes"`
	LocalPath string `json:"localPath,omitempty"`
}

// DeleteResult represents the result of a successful object deletion
type DeleteResult struct {
	Key    string `json:"key"`
	Region string `json:"region,omitempty"`
}
