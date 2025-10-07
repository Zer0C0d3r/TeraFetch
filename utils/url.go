package utils

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"terafetch/internal"
)

// URLInfo contains parsed information from a Terabox URL
type URLInfo struct {
	OriginalURL string
	Domain      string
	Surl        string
	ShareID     string
	IsPrivate   bool
}

// URLValidator handles URL validation and parsing for Terabox links
type URLValidator struct {
	allowedDomains []string
	urlPatterns    []*regexp.Regexp
}

// NewURLValidator creates a new URL validator with predefined patterns
func NewURLValidator() *URLValidator {
	allowedDomains := []string{
		"terabox.com",
		"www.terabox.com",
		"terabox.app",
		"www.terabox.app",
		"1024terabox.com",
		"www.1024terabox.com",
		"dm.terabox.com",
		"v1.terabox.com",
		"d.terabox.com",
		"data.terabox.com",
		"v2.terabox.com",
		"t.terabox.com",
		"autodiscover.terabox.com",
		"blog.terabox.com",
		"us.terabox.com",
		"jp.terabox.com",
		"ca.terabox.com",
		"email.terabox.com",
		"s.terabox.com",
		"op.terabox.com",
		"pan.baidu.com",
		"www.pan.baidu.com",
	}

	// Regex patterns for different Terabox URL formats
	patterns := []*regexp.Regexp{
		// Standard share URL: https://[subdomain.]terabox.com/s/1AbC123
		regexp.MustCompile(`^https?://(?:(?:www|dm|v1|v2|d|data|t|autodiscover|blog|us|jp|ca|email|s|op|1024terabox)\.)?terabox\.(?:com|app)/s/([a-zA-Z0-9_-]+)(?:\?.*)?$`),
		
		// Share URL with path: https://[subdomain.]terabox.com/sharing/link?surl=AbC123
		regexp.MustCompile(`^https?://(?:(?:www|dm|v1|v2|d|data|t|autodiscover|blog|us|jp|ca|email|s|op|1024terabox)\.)?terabox\.(?:com|app)/sharing/link\?.*surl=([a-zA-Z0-9_-]+)(?:&.*)?$`),
		
		// Terabox app sharing URL: https://terabox.app/sharing/link?surl=AbC123
		regexp.MustCompile(`^https?://(?:www\.)?terabox\.app/sharing/link\?.*surl=([a-zA-Z0-9_-]+)(?:&.*)?$`),
		
		// 1024terabox redirect URL: https://1024terabox.com/s/1AbC123
		regexp.MustCompile(`^https?://(?:www\.)?1024terabox\.com/s/([a-zA-Z0-9_-]+)(?:\?.*)?$`),
		
		// Baidu pan URL: https://pan.baidu.com/s/1AbC123
		regexp.MustCompile(`^https?://(?:www\.)?pan\.baidu\.com/s/([a-zA-Z0-9_-]+)(?:\?.*)?$`),
		
		// Baidu share URL: https://pan.baidu.com/share/link?shareid=123&uk=456
		regexp.MustCompile(`^https?://(?:www\.)?pan\.baidu\.com/share/link\?.*shareid=([0-9]+)(?:&.*)?$`),
		
		// Private file URL: https://[subdomain.]terabox.com/web/share/link?surl=AbC123&path=/folder
		regexp.MustCompile(`^https?://(?:(?:www|dm|v1|v2|d|data|t|autodiscover|blog|us|jp|ca|email|s|op)\.)?terabox\.(?:com|app)/web/share/link\?.*surl=([a-zA-Z0-9_-]+)(?:&.*)?$`),
	}

	return &URLValidator{
		allowedDomains: allowedDomains,
		urlPatterns:    patterns,
	}
}

// ValidateURL validates if the URL is from an allowed domain
func (v *URLValidator) ValidateURL(rawURL string) error {
	if rawURL == "" {
		return internal.NewValidationError("url", "URL cannot be empty")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return internal.NewValidationError("url", fmt.Sprintf("invalid URL format: %v", err))
	}

	// Check if the scheme is http or https
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return internal.NewValidationError("url", "URL must use http or https protocol")
	}

	// Normalize the host (remove port if present)
	host := strings.ToLower(parsedURL.Hostname())
	
	// Check if the domain is allowed
	for _, allowedDomain := range v.allowedDomains {
		if host == allowedDomain {
			return nil
		}
	}

	return internal.NewTeraboxError(
		0,
		fmt.Sprintf("URL must be from terabox.com or pan.baidu.com domains, got: %s", host),
		internal.ErrInvalidURL,
	)
}

// ParseURL extracts surl and shareid parameters from Terabox URLs
func (v *URLValidator) ParseURL(rawURL string) (*URLInfo, error) {
	// First validate the URL
	if err := v.ValidateURL(rawURL); err != nil {
		return nil, err
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, internal.NewValidationError("url", fmt.Sprintf("failed to parse URL: %v", err))
	}

	urlInfo := &URLInfo{
		OriginalURL: rawURL,
		Domain:      strings.ToLower(parsedURL.Hostname()),
	}

	// Try to match against known patterns
	for _, pattern := range v.urlPatterns {
		matches := pattern.FindStringSubmatch(rawURL)
		if len(matches) > 1 {
			// Extract the captured group (surl or shareid)
			captured := matches[1]
			
			// Determine if this is a surl or shareid based on the pattern and content
			if v.isShareID(captured) {
				urlInfo.ShareID = captured
			} else {
				urlInfo.Surl = captured
			}
			
			// Check for additional parameters in query string
			v.extractQueryParams(parsedURL, urlInfo)
			
			return urlInfo, nil
		}
	}

	// If no pattern matched, try to extract from query parameters
	v.extractQueryParams(parsedURL, urlInfo)
	
	// Validate that we extracted at least one identifier
	if urlInfo.Surl == "" && urlInfo.ShareID == "" {
		return nil, internal.NewTeraboxError(
			0,
			"unable to extract surl or shareid from URL",
			internal.ErrInvalidURL,
		)
	}

	return urlInfo, nil
}

// isShareID determines if a string looks like a numeric share ID
func (v *URLValidator) isShareID(s string) bool {
	// ShareIDs are typically numeric
	matched, _ := regexp.MatchString(`^\d+$`, s)
	return matched
}

// extractQueryParams extracts surl and shareid from URL query parameters
func (v *URLValidator) extractQueryParams(parsedURL *url.URL, urlInfo *URLInfo) {
	query := parsedURL.Query()
	
	// Extract surl parameter
	if surl := query.Get("surl"); surl != "" {
		urlInfo.Surl = surl
	}
	
	// Extract shareid parameter
	if shareid := query.Get("shareid"); shareid != "" {
		urlInfo.ShareID = shareid
	}
	
	// Check for path parameter (indicates private file)
	if path := query.Get("path"); path != "" && path != "/" {
		urlInfo.IsPrivate = true
	}
	
	// Check for uk parameter (user key, indicates private access)
	if uk := query.Get("uk"); uk != "" {
		urlInfo.IsPrivate = true
	}
}

// GetShareURL normalizes a URL to a standard share format
func (v *URLValidator) GetShareURL(urlInfo *URLInfo) string {
	if urlInfo.Surl != "" {
		// Use terabox.com as the canonical domain
		return fmt.Sprintf("https://terabox.com/s/%s", urlInfo.Surl)
	}
	
	if urlInfo.ShareID != "" {
		// For Baidu pan URLs with shareid
		return fmt.Sprintf("https://pan.baidu.com/share/link?shareid=%s", urlInfo.ShareID)
	}
	
	return urlInfo.OriginalURL
}

// IsPublicShare determines if the URL represents a public share
func (urlInfo *URLInfo) IsPublicShare() bool {
	return !urlInfo.IsPrivate && (urlInfo.Surl != "" || urlInfo.ShareID != "")
}

// GetIdentifier returns the primary identifier (surl or shareid)
func (urlInfo *URLInfo) GetIdentifier() string {
	if urlInfo.Surl != "" {
		return urlInfo.Surl
	}
	return urlInfo.ShareID
}

// String returns a string representation of the URLInfo
func (urlInfo *URLInfo) String() string {
	return fmt.Sprintf("URLInfo{Domain: %s, Surl: %s, ShareID: %s, IsPrivate: %t}", 
		urlInfo.Domain, urlInfo.Surl, urlInfo.ShareID, urlInfo.IsPrivate)
}