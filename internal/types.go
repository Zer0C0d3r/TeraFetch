package internal

import (
	"net/http"
	"time"
)

// FileMetadata contains information about a file to be downloaded
type FileMetadata struct {
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	DirectURL string    `json:"direct_url"`
	ShareID   string    `json:"share_id"`
	Timestamp time.Time `json:"timestamp"`
	Checksum  string    `json:"checksum,omitempty"`
}

// DownloadConfig contains configuration for download operations
type DownloadConfig struct {
	OutputPath string
	Threads    int
	RateLimit  int64 // bytes per second
	ProxyURL   string
	Quiet      bool
	ResumeData *ResumeMetadata
}

// AuthContext contains authentication information for Terabox
type AuthContext struct {
	Cookies   map[string]*http.Cookie
	BDUSS     string
	STOKEN    string
	ExpiresAt time.Time
	UserAgent string
	Bypass    bool // Indicates if this is a bypass attempt without real authentication
}

// SegmentInfo represents a download segment for multi-threaded downloads
type SegmentInfo struct {
	Index     int   `json:"index"`
	Start     int64 `json:"start"`
	End       int64 `json:"end"`
	Completed bool  `json:"completed"`
	Retries   int   `json:"retries"`
}

// ResumeMetadata contains information needed to resume interrupted downloads
type ResumeMetadata struct {
	FileMetadata *FileMetadata `json:"file_metadata"`
	Segments     []SegmentInfo `json:"segments"`
	CreatedAt    time.Time     `json:"created_at"`
	LastUpdate   time.Time     `json:"last_update"`
}