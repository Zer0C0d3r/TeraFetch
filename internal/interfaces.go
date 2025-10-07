package internal

import "context"

// LinkResolver handles Terabox URL resolution
type LinkResolver interface {
	ResolvePublicLink(url string) (*FileMetadata, error)
	ResolvePrivateLink(url string, auth *AuthContext) (*FileMetadata, error)
}

// DownloadEngine manages multi-threaded downloads
type DownloadEngine interface {
	Download(meta *FileMetadata, config *DownloadConfig) error
	Resume(partialPath string, config *DownloadConfig) error
}

// AuthManager handles authentication and session management
type AuthManager interface {
	LoadCookies(path string) (*AuthContext, error)
	ValidateSession(auth *AuthContext) error
	RefreshSession(auth *AuthContext) error
}

// RateLimiter controls bandwidth usage
type RateLimiter interface {
	Wait(ctx context.Context, n int) error
	SetRate(bytesPerSecond int64)
}