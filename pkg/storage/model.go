package storage

import "time"

// ObjectInfo represents metadata about a single object
type ObjectInfo struct {
	Key          string    `json:"key"`
	ContentType  string    `json:"contentType"`
	SizeBytes    int64     `json:"sizeBytes"`
	LastModified time.Time `json:"lastModified"`
}

// ListResult represents the result of listing objects
type ListResult struct {
	Objects []ObjectInfo `json:"objects"`
	Cursor  string       `json:"cursor,omitempty"`
}

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
