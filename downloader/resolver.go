package downloader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"terafetch/internal"
	"terafetch/utils"
)

// TeraboxResolver implements the LinkResolver interface
type TeraboxResolver struct {
	httpClient   *utils.HTTPClient
	urlValidator *utils.URLValidator
}

// TeraboxAPIResponse represents the common structure of Terabox API responses
type TeraboxAPIResponse struct {
	Errno  int    `json:"errno"`
	Errmsg string `json:"errmsg"`
}

// ShareDownloadResponse represents the response from sharedownload API
type ShareDownloadResponse struct {
	TeraboxAPIResponse
	Dlink    string `json:"dlink"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MD5      string `json:"md5"`
}

// FileMetasResponse represents the response from filemetas API
type FileMetasResponse struct {
	TeraboxAPIResponse
	List []FileInfo `json:"list"`
}

// FileInfo represents file information from filemetas API
type FileInfo struct {
	Filename   string `json:"server_filename"`
	Size       int64  `json:"size"`
	MD5        string `json:"md5"`
	FsID       int64  `json:"fs_id"`
	Path       string `json:"path"`
	IsDir      int    `json:"isdir"`
	ServerMD5  string `json:"server_md5"`
	Category   int    `json:"category"`
	CreateTime int64  `json:"server_ctime"`
	ModTime    int64  `json:"server_mtime"`
}

// DownloadResponse represents the response from download API
type DownloadResponse struct {
	TeraboxAPIResponse
	Dlink string `json:"dlink"`
}

// NewTeraboxResolver creates a new instance of TeraboxResolver
func NewTeraboxResolver() *TeraboxResolver {
	return &TeraboxResolver{
		httpClient:   utils.NewHTTPClient(),
		urlValidator: utils.NewURLValidator(),
	}
}

// NewTeraboxResolverWithClient creates a new instance with custom HTTP client
func NewTeraboxResolverWithClient(httpClient *utils.HTTPClient) *TeraboxResolver {
	return &TeraboxResolver{
		httpClient:   httpClient,
		urlValidator: utils.NewURLValidator(),
	}
}

// ResolvePublicLink resolves a public Terabox share URL to download metadata
func (r *TeraboxResolver) ResolvePublicLink(url string) (*internal.FileMetadata, error) {
	// Parse and validate the URL
	urlInfo, err := r.urlValidator.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Check if this is actually a public share
	if !urlInfo.IsPublicShare() {
		return nil, internal.NewTeraboxError(0, "URL appears to be a private share, use ResolvePrivateLink instead", internal.ErrAuthRequired)
	}

	// Call the sharedownload API
	return r.callShareDownloadAPI(urlInfo)
}

// ResolvePrivateLink resolves a private Terabox URL using authentication
func (r *TeraboxResolver) ResolvePrivateLink(url string, auth *internal.AuthContext) (*internal.FileMetadata, error) {
	if auth == nil {
		return nil, internal.NewTeraboxError(0, "authentication context is required for private links", internal.ErrAuthRequired)
	}

	// Parse and validate the URL
	urlInfo, err := r.urlValidator.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// First, get file metadata using filemetas API
	fileInfo, err := r.callFileMetasAPI(urlInfo, auth)
	if err != nil {
		return nil, fmt.Errorf("failed to get file metadata: %w", err)
	}

	// Then get the download link using download API
	dlink, err := r.callDownloadAPI(fileInfo, auth)
	if err != nil {
		return nil, fmt.Errorf("failed to get download link: %w", err)
	}

	// Create FileMetadata from the results
	return &internal.FileMetadata{
		Filename:  fileInfo.Filename,
		Size:      fileInfo.Size,
		DirectURL: dlink,
		ShareID:   urlInfo.GetIdentifier(),
		Timestamp: time.Now(),
		Checksum:  fileInfo.MD5,
	}, nil
}

// ResolveWithBypass attempts to resolve URLs using bypass techniques
func (r *TeraboxResolver) ResolveWithBypass(url string) (*internal.FileMetadata, error) {
	// Parse and validate the URL
	urlInfo, err := r.urlValidator.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Try multiple bypass approaches
	approaches := []struct {
		name string
		fn   func(*utils.URLInfo) (*internal.FileMetadata, error)
	}{
		{"Direct Share API", r.tryDirectShareAPI},
		{"Alternative API", r.tryAlternativeAPI},
		{"Web Scraping", r.tryWebScraping},
	}

	var errors []string
	for i, approach := range approaches {
		fmt.Printf("🔄 Trying %s...\n", approach.name)
		metadata, err := approach.fn(urlInfo)
		if err == nil {
			fmt.Printf("✅ %s succeeded!\n", approach.name)
			return metadata, nil
		}
		
		errorMsg := fmt.Sprintf("%s failed: %v", approach.name, err)
		errors = append(errors, errorMsg)
		fmt.Printf("❌ %s\n", errorMsg)
		
		// Add delay between attempts to avoid rate limiting
		if i < len(approaches)-1 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	return nil, fmt.Errorf("all bypass approaches failed:\n%s", strings.Join(errors, "\n"))
}

// tryDirectShareAPI attempts to use the share API with different parameters
func (r *TeraboxResolver) tryDirectShareAPI(urlInfo *utils.URLInfo) (*internal.FileMetadata, error) {
	// Try different API endpoints and parameters
	endpoints := []string{
		"https://www.terabox.com/api/sharedownload",
		"https://terabox.com/api/sharedownload", 
		"https://www.terabox.app/api/sharedownload",
	}

	for _, endpoint := range endpoints {
		params := url.Values{}
		if urlInfo.Surl != "" {
			params.Set("surl", urlInfo.Surl)
		} else if urlInfo.ShareID != "" {
			params.Set("shareid", urlInfo.ShareID)
		}
		
		// Try different parameter combinations
		paramSets := []url.Values{
			// Standard parameters
			func() url.Values {
				p := params
				p.Set("channel", "chunlei")
				p.Set("web", "1")
				p.Set("app_id", "250528")
				p.Set("clienttype", "0")
				return p
			}(),
			// Alternative parameters
			func() url.Values {
				p := params
				p.Set("channel", "dubox")
				p.Set("web", "1")
				p.Set("app_id", "778750")
				p.Set("clienttype", "5")
				return p
			}(),
		}

		for _, paramSet := range paramSets {
			fullURL := fmt.Sprintf("%s?%s", endpoint, paramSet.Encode())
			
			headers := map[string]string{
				"Referer":          "https://www.terabox.com/",
				"Origin":           "https://www.terabox.com",
				"X-Requested-With": "XMLHttpRequest",
			}

			resp, err := r.httpClient.GetWithHeaders(fullURL, headers)
			if err != nil {
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				continue
			}

			var apiResp ShareDownloadResponse
			if err := json.Unmarshal(body, &apiResp); err != nil {
				continue
			}

			// If we get a successful response, return it
			if apiResp.Errno == 0 && apiResp.Dlink != "" {
				return &internal.FileMetadata{
					Filename:  apiResp.Filename,
					Size:      apiResp.Size,
					DirectURL: apiResp.Dlink,
					ShareID:   urlInfo.GetIdentifier(),
					Timestamp: time.Now(),
					Checksum:  apiResp.MD5,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("direct share API approaches failed")
}

// tryAlternativeAPI attempts to use alternative API endpoints
func (r *TeraboxResolver) tryAlternativeAPI(urlInfo *utils.URLInfo) (*internal.FileMetadata, error) {
	// Try the list API which sometimes works without authentication
	apiURL := "https://www.terabox.com/api/list"
	
	params := url.Values{}
	if urlInfo.Surl != "" {
		params.Set("surl", urlInfo.Surl)
	}
	params.Set("dir", "/")
	params.Set("num", "100")
	params.Set("order", "name")
	params.Set("desc", "0")
	params.Set("web", "1")
	params.Set("app_id", "250528")

	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	headers := map[string]string{
		"Referer": "https://www.terabox.com/",
		"Origin":  "https://www.terabox.com",
	}

	resp, err := r.httpClient.GetWithHeaders(fullURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to call list API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var listResp FileMetasResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if listResp.Errno != 0 || len(listResp.List) == 0 {
		return nil, fmt.Errorf("list API returned no files")
	}

	// Use the first file from the list
	fileInfo := listResp.List[0]
	
	// Try to get direct download link
	dlink, err := r.tryGetDirectLink(fileInfo.FsID, urlInfo.Surl)
	if err != nil {
		return nil, fmt.Errorf("failed to get direct link: %w", err)
	}

	return &internal.FileMetadata{
		Filename:  fileInfo.Filename,
		Size:      fileInfo.Size,
		DirectURL: dlink,
		ShareID:   urlInfo.GetIdentifier(),
		Timestamp: time.Now(),
		Checksum:  fileInfo.MD5,
	}, nil
}

// tryWebScraping attempts to extract download links from the web page
func (r *TeraboxResolver) tryWebScraping(urlInfo *utils.URLInfo) (*internal.FileMetadata, error) {
	// Get the share page URL
	shareURL := fmt.Sprintf("https://terabox.com/s/%s", urlInfo.GetIdentifier())
	
	// Try different approaches to get the page content
	approaches := []struct {
		name string
		fn   func(string) (*internal.FileMetadata, error)
	}{
		{"Share Page", r.scrapeSharePage},
		{"Redirect Follow", r.scrapeWithRedirect},
		{"Alternative Domains", r.scrapeAlternativeDomains},
	}
	
	var errors []string
	for _, approach := range approaches {
		fmt.Printf("  🔄 Trying %s scraping...\n", approach.name)
		metadata, err := approach.fn(shareURL)
		if err == nil {
			fmt.Printf("  ✅ %s scraping succeeded!\n", approach.name)
			return metadata, nil
		}
		
		errorMsg := fmt.Sprintf("%s: %v", approach.name, err)
		errors = append(errors, errorMsg)
		fmt.Printf("  ❌ %s\n", errorMsg)
	}
	
	return nil, fmt.Errorf("web scraping failed: %s", strings.Join(errors, "; "))
}

// scrapeSharePage attempts to scrape the main share page
func (r *TeraboxResolver) scrapeSharePage(shareURL string) (*internal.FileMetadata, error) {
	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Accept":     "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.5",
		"Cache-Control": "no-cache",
		"Pragma": "no-cache",
	}
	
	resp, err := r.httpClient.GetWithHeaders(shareURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch share page: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read page content: %w", err)
	}
	
	content := string(body)
	
	// Try to extract file information from various sources in the HTML
	metadata := &internal.FileMetadata{
		Timestamp: time.Now(),
	}
	
	// Extract filename from meta tags or page content
	if filename := r.extractFilename(content); filename != "" {
		metadata.Filename = filename
	}
	
	// Extract file size from page content
	if size := r.extractFileSize(content); size > 0 {
		metadata.Size = size
	}
	
	// Try to extract direct download link
	if dlink := r.extractDownloadLink(content); dlink != "" {
		metadata.DirectURL = dlink
	}
	
	// Extract share ID
	if shareID := r.extractShareID(content); shareID != "" {
		metadata.ShareID = shareID
	}
	
	// Check if the page indicates authentication is required
	if strings.Contains(content, "extraction code") || strings.Contains(content, "password") {
		return nil, fmt.Errorf("file requires password/extraction code - this is a private share")
	}
	
	// Check if the file is expired or not found
	if strings.Contains(content, "not found") || strings.Contains(content, "expired") {
		return nil, fmt.Errorf("file not found or expired")
	}
	
	// If we found a filename, that's at least something
	if metadata.Filename != "" {
		// Extract share ID from the URL if not already set
		if metadata.ShareID == "" {
			if parts := strings.Split(shareURL, "/s/"); len(parts) > 1 {
				metadata.ShareID = parts[1]
			}
		}
		
		// If we don't have a direct download link, we can still provide file info
		if metadata.DirectURL == "" {
			return nil, fmt.Errorf("found file '%s' but no direct download link available - may require authentication", metadata.Filename)
		}
		
		return metadata, nil
	}
	
	return nil, fmt.Errorf("could not extract file information from page")
}

// scrapeWithRedirect follows redirects and tries to scrape the final page
func (r *TeraboxResolver) scrapeWithRedirect(shareURL string) (*internal.FileMetadata, error) {
	// Try following the redirect chain manually
	redirectURL := shareURL
	maxRedirects := 5
	
	for i := 0; i < maxRedirects; i++ {
		headers := map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		}
		
		resp, err := r.httpClient.GetWithHeaders(redirectURL, headers)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch redirect page: %w", err)
		}
		
		// Check if this is a redirect
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			location := resp.Header.Get("Location")
			resp.Body.Close()
			if location == "" {
				break
			}
			redirectURL = location
			continue
		}
		
		// Try to scrape this page
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read redirect page content: %w", err)
		}
		
		content := string(body)
		
		// Look for JavaScript variables that might contain file info
		if metadata := r.extractFromJavaScript(content); metadata != nil {
			return metadata, nil
		}
		
		break
	}
	
	return nil, fmt.Errorf("redirect scraping failed")
}

// scrapeAlternativeDomains tries scraping from alternative domains
func (r *TeraboxResolver) scrapeAlternativeDomains(shareURL string) (*internal.FileMetadata, error) {
	// Extract share ID from URL
	shareID := ""
	if parts := strings.Split(shareURL, "/s/"); len(parts) > 1 {
		shareID = parts[1]
	}
	
	if shareID == "" {
		return nil, fmt.Errorf("could not extract share ID")
	}
	
	// Try different domain variations
	domains := []string{
		"www.terabox.com",
		"terabox.app",
		"www.terabox.app",
	}
	
	for _, domain := range domains {
		altURL := fmt.Sprintf("https://%s/sharing/link?surl=%s", domain, shareID)
		
		headers := map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Referer":    fmt.Sprintf("https://%s/", domain),
		}
		
		resp, err := r.httpClient.GetWithHeaders(altURL, headers)
		if err != nil {
			continue
		}
		
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}
		
		content := string(body)
		
		// Try to extract file information
		if metadata := r.extractFromJavaScript(content); metadata != nil {
			return metadata, nil
		}
	}
	
	return nil, fmt.Errorf("alternative domain scraping failed")
}

// Helper methods for extracting information from HTML content

func (r *TeraboxResolver) extractFilename(content string) string {
	// Try multiple patterns to extract filename
	patterns := []string{
		`<meta name="description" content="([^"]*?)\.([^"]*?) -`,
		`<title>([^<]*?)</title>`,
		`"server_filename":"([^"]*?)"`,
		`filename['"]\s*:\s*['"]([^'"]*?)['"]`,
		`name['"]\s*:\s*['"]([^'"]*?)['"]`,
	}
	
	for _, pattern := range patterns {
		if matches := regexp.MustCompile(pattern).FindStringSubmatch(content); len(matches) > 1 {
			filename := strings.TrimSpace(matches[1])
			if filename != "" && !strings.Contains(filename, "TeraBox") {
				// Add extension if found in second capture group
				if len(matches) > 2 && matches[2] != "" {
					filename += "." + matches[2]
				}
				return filename
			}
		}
	}
	
	return ""
}

func (r *TeraboxResolver) extractFileSize(content string) int64 {
	// Try to extract file size from various patterns
	patterns := []string{
		`"size":(\d+)`,
		`size['"]\s*:\s*(\d+)`,
		`filesize['"]\s*:\s*(\d+)`,
	}
	
	for _, pattern := range patterns {
		if matches := regexp.MustCompile(pattern).FindStringSubmatch(content); len(matches) > 1 {
			if size, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
				return size
			}
		}
	}
	
	return 0
}

func (r *TeraboxResolver) extractDownloadLink(content string) string {
	// Try to extract download links from various patterns
	patterns := []string{
		`"dlink":"([^"]*?)"`,
		`dlink['"]\s*:\s*['"]([^'"]*?)['"]`,
		`download_url['"]\s*:\s*['"]([^'"]*?)['"]`,
		`href=['"]([^'"]*?download[^'"]*?)['"]`,
	}
	
	for _, pattern := range patterns {
		if matches := regexp.MustCompile(pattern).FindStringSubmatch(content); len(matches) > 1 {
			link := strings.TrimSpace(matches[1])
			if link != "" && (strings.Contains(link, "http") || strings.HasPrefix(link, "//")) {
				// Clean up the link
				link = strings.ReplaceAll(link, "\\", "")
				if strings.HasPrefix(link, "//") {
					link = "https:" + link
				}
				return link
			}
		}
	}
	
	return ""
}

func (r *TeraboxResolver) extractShareID(content string) string {
	// Try to extract share ID from various patterns
	patterns := []string{
		`"surl":"([^"]*?)"`,
		`surl['"]\s*:\s*['"]([^'"]*?)['"]`,
		`share_id['"]\s*:\s*['"]([^'"]*?)['"]`,
	}
	
	for _, pattern := range patterns {
		if matches := regexp.MustCompile(pattern).FindStringSubmatch(content); len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}
	
	return ""
}

func (r *TeraboxResolver) extractFromJavaScript(content string) *internal.FileMetadata {
	// Look for JavaScript objects that might contain file information
	jsPatterns := []string{
		`window\.yunData\s*=\s*({[^}]*?})`,
		`var\s+yunData\s*=\s*({[^}]*?})`,
		`fileInfo\s*:\s*({[^}]*?})`,
		`shareInfo\s*:\s*({[^}]*?})`,
	}
	
	for _, pattern := range jsPatterns {
		if matches := regexp.MustCompile(pattern).FindStringSubmatch(content); len(matches) > 1 {
			jsData := matches[1]
			
			// Try to extract information from the JavaScript object
			metadata := &internal.FileMetadata{
				Timestamp: time.Now(),
			}
			
			if filename := r.extractFilename(jsData); filename != "" {
				metadata.Filename = filename
			}
			
			if size := r.extractFileSize(jsData); size > 0 {
				metadata.Size = size
			}
			
			if dlink := r.extractDownloadLink(jsData); dlink != "" {
				metadata.DirectURL = dlink
			}
			
			if shareID := r.extractShareID(jsData); shareID != "" {
				metadata.ShareID = shareID
			}
			
			// If we found useful information, return it
			if metadata.Filename != "" || metadata.DirectURL != "" {
				return metadata
			}
		}
	}
	
	return nil
}

// tryGetDirectLink attempts to get a direct download link using file ID
func (r *TeraboxResolver) tryGetDirectLink(fsID int64, surl string) (string, error) {
	apiURL := "https://www.terabox.com/api/download"
	
	params := url.Values{}
	params.Set("fidlist", fmt.Sprintf("[%d]", fsID))
	params.Set("surl", surl)
	params.Set("web", "1")
	params.Set("app_id", "250528")

	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	headers := map[string]string{
		"Referer": "https://www.terabox.com/",
		"Origin":  "https://www.terabox.com",
	}

	resp, err := r.httpClient.GetWithHeaders(fullURL, headers)
	if err != nil {
		return "", fmt.Errorf("failed to call download API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var downloadResp map[string]interface{}
	if err := json.Unmarshal(body, &downloadResp); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Extract download link from response
	if dlist, ok := downloadResp["dlist"].([]interface{}); ok && len(dlist) > 0 {
		if file, ok := dlist[0].(map[string]interface{}); ok {
			if dlink, ok := file["dlink"].(string); ok && dlink != "" {
				return dlink, nil
			}
		}
	}

	return "", fmt.Errorf("no download link found in response")
}

// callShareDownloadAPI calls the Terabox sharedownload API for public links
func (r *TeraboxResolver) callShareDownloadAPI(urlInfo *utils.URLInfo) (*internal.FileMetadata, error) {
	// Construct the API URL
	apiURL := "https://www.terabox.com/api/sharedownload"
	
	// Prepare query parameters
	params := url.Values{}
	if urlInfo.Surl != "" {
		params.Set("surl", urlInfo.Surl)
	} else if urlInfo.ShareID != "" {
		params.Set("shareid", urlInfo.ShareID)
	}
	params.Set("channel", "chunlei")
	params.Set("web", "1")
	params.Set("app_id", "250528")
	params.Set("clienttype", "0")

	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	// Prepare headers
	headers := map[string]string{
		"Referer":    "https://www.terabox.com/",
		"Origin":     "https://www.terabox.com",
		"X-Requested-With": "XMLHttpRequest",
	}

	// Make the API request
	resp, err := r.httpClient.GetWithHeaders(fullURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to call sharedownload API: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON response
	var apiResp ShareDownloadResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Check for API errors
	if err := r.handleAPIError(apiResp.TeraboxAPIResponse); err != nil {
		return nil, err
	}

	// Validate that we got a download link
	if apiResp.Dlink == "" {
		return nil, internal.NewTeraboxError(apiResp.Errno, "no download link found in response", internal.ErrInvalidResponse)
	}

	// Create FileMetadata
	return &internal.FileMetadata{
		Filename:  apiResp.Filename,
		Size:      apiResp.Size,
		DirectURL: apiResp.Dlink,
		ShareID:   urlInfo.GetIdentifier(),
		Timestamp: time.Now(),
		Checksum:  apiResp.MD5,
	}, nil
}

// callFileMetasAPI calls the Terabox filemetas API for private links
func (r *TeraboxResolver) callFileMetasAPI(urlInfo *utils.URLInfo, auth *internal.AuthContext) (*FileInfo, error) {
	// Construct the API URL
	apiURL := "https://www.terabox.com/api/filemetas"
	
	// Prepare query parameters
	params := url.Values{}
	if urlInfo.Surl != "" {
		params.Set("surl", urlInfo.Surl)
	} else if urlInfo.ShareID != "" {
		params.Set("shareid", urlInfo.ShareID)
	}
	params.Set("channel", "chunlei")
	params.Set("web", "1")
	params.Set("app_id", "250528")
	params.Set("clienttype", "0")
	params.Set("dir", "1") // Get directory listing

	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	// Prepare headers with authentication
	headers := map[string]string{
		"Referer":    "https://www.terabox.com/",
		"Origin":     "https://www.terabox.com",
		"X-Requested-With": "XMLHttpRequest",
	}

	// Add authentication cookies to the request
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Add cookies
	for _, cookie := range auth.Cookies {
		req.AddCookie(cookie)
	}

	// Make the request using the HTTP client's underlying client
	// Note: We need to use the underlying client directly for cookie support
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call filemetas API: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON response
	var apiResp FileMetasResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Check for API errors
	if err := r.handleAPIError(apiResp.TeraboxAPIResponse); err != nil {
		return nil, err
	}

	// Find the first file (not directory) in the response
	for _, file := range apiResp.List {
		if file.IsDir == 0 { // 0 means it's a file, not a directory
			return &file, nil
		}
	}

	return nil, internal.NewTeraboxError(apiResp.Errno, "no files found in share", internal.ErrFileNotFound)
}

// callDownloadAPI calls the Terabox download API to get the direct download link
func (r *TeraboxResolver) callDownloadAPI(fileInfo *FileInfo, auth *internal.AuthContext) (string, error) {
	// Construct the API URL
	apiURL := "https://www.terabox.com/api/download"
	
	// Prepare query parameters
	params := url.Values{}
	params.Set("fidlist", fmt.Sprintf("[%d]", fileInfo.FsID))
	params.Set("channel", "chunlei")
	params.Set("web", "1")
	params.Set("app_id", "250528")
	params.Set("clienttype", "0")

	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	// Prepare headers with authentication
	headers := map[string]string{
		"Referer":    "https://www.terabox.com/",
		"Origin":     "https://www.terabox.com",
		"X-Requested-With": "XMLHttpRequest",
	}

	// Add authentication cookies to the request
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Add cookies
	for _, cookie := range auth.Cookies {
		req.AddCookie(cookie)
	}

	// Make the request using a basic HTTP client for cookie support
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call download API: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON response
	var apiResp DownloadResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Check for API errors
	if err := r.handleAPIError(apiResp.TeraboxAPIResponse); err != nil {
		return "", err
	}

	// Validate that we got a download link
	if apiResp.Dlink == "" {
		return "", internal.NewTeraboxError(apiResp.Errno, "no download link found in response", internal.ErrInvalidResponse)
	}

	return apiResp.Dlink, nil
}

// handleAPIError processes Terabox API error responses and returns appropriate errors
func (r *TeraboxResolver) handleAPIError(apiResp TeraboxAPIResponse) error {
	if apiResp.Errno == 0 {
		return nil // Success
	}

	// Map common Terabox error codes to our error types
	switch apiResp.Errno {
	case -1:
		return internal.NewTeraboxError(apiResp.Errno, "invalid request parameters", internal.ErrInvalidURL)
	case -2:
		return internal.NewTeraboxError(apiResp.Errno, "authentication required or invalid", internal.ErrAuthRequired)
	case -3:
		return internal.NewTeraboxError(apiResp.Errno, "access denied", internal.ErrAuthRequired)
	case -4:
		return internal.NewTeraboxError(apiResp.Errno, "file not found or share expired", internal.ErrFileNotFound)
	case -5:
		return internal.NewTeraboxError(apiResp.Errno, "share link invalid or expired", internal.ErrInvalidURL)
	case -6:
		return internal.NewTeraboxError(apiResp.Errno, "rate limit exceeded", internal.ErrRateLimit)
	case -7:
		return internal.NewTeraboxError(apiResp.Errno, "quota exceeded", internal.ErrQuotaExceeded)
	case -8:
		return internal.NewTeraboxError(apiResp.Errno, "file too large", internal.ErrQuotaExceeded)
	case -9:
		return internal.NewTeraboxError(apiResp.Errno, "anti-bot verification required", internal.ErrRateLimit)
	case -10:
		return internal.NewTeraboxError(apiResp.Errno, "IP blocked or suspicious activity", internal.ErrRateLimit)
	case 2:
		return internal.NewTeraboxError(apiResp.Errno, "parameter error", internal.ErrInvalidURL)
	case 3:
		return internal.NewTeraboxError(apiResp.Errno, "method not allowed", internal.ErrInvalidURL)
	case 4:
		return internal.NewTeraboxError(apiResp.Errno, "user not found", internal.ErrAuthRequired)
	case 5:
		return internal.NewTeraboxError(apiResp.Errno, "user not activated", internal.ErrAuthRequired)
	case 6:
		return internal.NewTeraboxError(apiResp.Errno, "user forbidden", internal.ErrAuthRequired)
	case 7:
		return internal.NewTeraboxError(apiResp.Errno, "file or folder not found", internal.ErrFileNotFound)
	case 8:
		return internal.NewTeraboxError(apiResp.Errno, "file name illegal", internal.ErrInvalidURL)
	case 9:
		return internal.NewTeraboxError(apiResp.Errno, "file forbidden", internal.ErrAuthRequired)
	case 10:
		return internal.NewTeraboxError(apiResp.Errno, "share not found", internal.ErrFileNotFound)
	case 11:
		return internal.NewTeraboxError(apiResp.Errno, "share cancelled", internal.ErrFileNotFound)
	case 12:
		return internal.NewTeraboxError(apiResp.Errno, "share expired", internal.ErrFileNotFound)
	case 13:
		return internal.NewTeraboxError(apiResp.Errno, "share access denied", internal.ErrAuthRequired)
	case 14:
		return internal.NewTeraboxError(apiResp.Errno, "share password required", internal.ErrAuthRequired)
	case 15:
		return internal.NewTeraboxError(apiResp.Errno, "share password incorrect", internal.ErrAuthRequired)
	case 16:
		return internal.NewTeraboxError(apiResp.Errno, "share access limit exceeded", internal.ErrRateLimit)
	case 17:
		return internal.NewTeraboxError(apiResp.Errno, "share download limit exceeded", internal.ErrQuotaExceeded)
	case 18:
		return internal.NewTeraboxError(apiResp.Errno, "share traffic limit exceeded", internal.ErrQuotaExceeded)
	case 110:
		return internal.NewTeraboxError(apiResp.Errno, "access token invalid", internal.ErrAuthRequired)
	case 111:
		return internal.NewTeraboxError(apiResp.Errno, "access token expired", internal.ErrAuthRequired)
	case 31034:
		return internal.NewTeraboxError(apiResp.Errno, "anti-crawler verification failed", internal.ErrRateLimit)
	case 31045:
		return internal.NewTeraboxError(apiResp.Errno, "verification code required", internal.ErrRateLimit)
	case 31061:
		return internal.NewTeraboxError(apiResp.Errno, "file download forbidden", internal.ErrAuthRequired)
	case 31062:
		return internal.NewTeraboxError(apiResp.Errno, "file access restricted", internal.ErrAuthRequired)
	case 31066:
		return internal.NewTeraboxError(apiResp.Errno, "file sharing disabled", internal.ErrAuthRequired)
	default:
		// Unknown error code
		message := apiResp.Errmsg
		if message == "" {
			message = fmt.Sprintf("unknown API error (code: %d)", apiResp.Errno)
		}
		return internal.NewTeraboxError(apiResp.Errno, message, internal.ErrInvalidResponse)
	}
}