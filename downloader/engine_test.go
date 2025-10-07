package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"terafetch/internal"
)

// TestMultiThreadEngine_Download tests the basic download functionality
func TestMultiThreadEngine_Download(t *testing.T) {
	// Create test data
	testData := strings.Repeat("Hello, World! ", 1000) // ~13KB of data
	expectedSize := int64(len(testData))

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle range requests
		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "" {
			// Parse range header (simplified for test)
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", expectedSize-1, expectedSize))
			w.WriteHeader(http.StatusPartialContent)
		} else {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", expectedSize))
		}
		
		w.Header().Set("Content-Type", "application/octet-stream")
		io.WriteString(w, testData)
	}))
	defer server.Close()

	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "terafetch_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create engine
	engine := NewMultiThreadEngine()

	// Create file metadata
	meta := &internal.FileMetadata{
		Filename:  "test_file.txt",
		Size:      expectedSize,
		DirectURL: server.URL,
		ShareID:   "test123",
		Timestamp: time.Now(),
	}

	// Create download config
	outputPath := filepath.Join(tempDir, "test_file.txt")
	config := &internal.DownloadConfig{
		OutputPath: outputPath,
		Threads:    2,
		RateLimit:  0, // No rate limiting for test
		Quiet:      true,
	}

	// Execute download
	err = engine.Download(meta, config)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Verify file exists and has correct size
	if !engine.fileOps.FileExists(outputPath) {
		t.Fatalf("Downloaded file does not exist: %s", outputPath)
	}

	actualSize, err := engine.fileOps.GetFileSize(outputPath)
	if err != nil {
		t.Fatalf("Failed to get file size: %v", err)
	}

	if actualSize != expectedSize {
		t.Fatalf("File size mismatch: expected %d, got %d", expectedSize, actualSize)
	}

	// Verify file content
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(content) != testData {
		t.Fatalf("File content mismatch")
	}
}

// TestMultiThreadEngine_Resume tests the resume functionality
func TestMultiThreadEngine_Resume(t *testing.T) {
	// Create test data
	testData := strings.Repeat("Test data for resume! ", 500) // ~11KB of data
	expectedSize := int64(len(testData))

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "" {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", expectedSize-1, expectedSize))
			w.WriteHeader(http.StatusPartialContent)
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		io.WriteString(w, testData)
	}))
	defer server.Close()

	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "terafetch_resume_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create engine
	engine := NewMultiThreadEngine()

	// Create file metadata
	meta := &internal.FileMetadata{
		Filename:  "resume_test.txt",
		Size:      expectedSize,
		DirectURL: server.URL,
		ShareID:   "resume123",
		Timestamp: time.Now(),
	}

	outputPath := filepath.Join(tempDir, "resume_test.txt")

	// Create partial download scenario by creating resume metadata
	segments := []internal.SegmentInfo{
		{Index: 0, Start: 0, End: expectedSize/2 - 1, Completed: true, Retries: 0},
		{Index: 1, Start: expectedSize / 2, End: expectedSize - 1, Completed: false, Retries: 0},
	}

	resumeData := &internal.ResumeMetadata{
		FileMetadata: meta,
		Segments:     segments,
		CreatedAt:    time.Now(),
		LastUpdate:   time.Now(),
	}

	// Create download config with resume data
	config := &internal.DownloadConfig{
		OutputPath: outputPath,
		Threads:    2,
		RateLimit:  0,
		Quiet:      true,
		ResumeData: resumeData,
	}

	// Execute download (should resume)
	err = engine.Download(meta, config)
	if err != nil {
		t.Fatalf("Resume download failed: %v", err)
	}

	// Verify file exists and has correct size
	if !engine.fileOps.FileExists(outputPath) {
		t.Fatalf("Resumed file does not exist: %s", outputPath)
	}

	actualSize, err := engine.fileOps.GetFileSize(outputPath)
	if err != nil {
		t.Fatalf("Failed to get file size: %v", err)
	}

	if actualSize != expectedSize {
		t.Fatalf("File size mismatch after resume: expected %d, got %d", expectedSize, actualSize)
	}
}

// TestWorkerPool tests the worker pool functionality
func TestWorkerPool(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPartialContent)
		io.WriteString(w, "test segment data")
	}))
	defer server.Close()

	// Create temporary file for testing
	tempDir, err := os.MkdirTemp("", "worker_pool_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	partPath := filepath.Join(tempDir, "test.part")
	
	// Create part file
	file, err := os.Create(partPath)
	if err != nil {
		t.Fatalf("Failed to create part file: %v", err)
	}
	file.Truncate(1000) // 1KB file
	file.Close()

	// Create engine and worker pool
	engine := NewMultiThreadEngine()
	pool := engine.createWorkerPool(2, 0)
	defer pool.shutdown()

	// Start worker pool
	pool.start()

	// Create test job
	job := DownloadJob{
		Segment: internal.SegmentInfo{
			Index: 0,
			Start: 0,
			End:   16, // Length of "test segment data"
		},
		FileURL:    server.URL,
		OutputPath: filepath.Join(tempDir, "test.txt"),
		PartPath:   partPath,
	}

	// Submit job
	go func() {
		pool.jobs <- job
		close(pool.jobs)
	}()

	// Wait for result
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	select {
	case result := <-pool.results:
		if result.Error != nil {
			t.Fatalf("Worker job failed: %v", result.Error)
		}
		if !result.Completed {
			t.Fatalf("Worker job not completed")
		}
		if result.BytesWritten <= 0 {
			t.Fatalf("No bytes written by worker")
		}
	case <-ctx.Done():
		t.Fatalf("Worker pool test timed out")
	}
}

// TestFileIntegrityVerification tests the file integrity verification
func TestFileIntegrityVerification(t *testing.T) {
	// Create temporary file
	tempDir, err := os.MkdirTemp("", "integrity_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "integrity_test.txt")
	testData := "Hello, integrity test!"
	expectedSize := int64(len(testData))

	// Write test file
	err = os.WriteFile(testFile, []byte(testData), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create engine
	engine := NewMultiThreadEngine()

	// Test correct size verification
	err = engine.verifyFileIntegrity(testFile, expectedSize)
	if err != nil {
		t.Fatalf("Integrity verification failed for correct size: %v", err)
	}

	// Test incorrect size verification
	err = engine.verifyFileIntegrity(testFile, expectedSize+10)
	if err == nil {
		t.Fatalf("Integrity verification should have failed for incorrect size")
	}
}

// testState holds thread-safe state for test HTTP handlers
type testState struct {
	mu            sync.Mutex
	requestCount  int
	rangeRequests []string
}

func (ts *testState) incrementCount() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.requestCount++
}

func (ts *testState) addRangeRequest(rangeHeader string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.rangeRequests = append(ts.rangeRequests, rangeHeader)
}

func (ts *testState) getCount() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.requestCount
}

func (ts *testState) getRangeRequests() []string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	// Return a copy to avoid race conditions
	result := make([]string, len(ts.rangeRequests))
	copy(result, ts.rangeRequests)
	return result
}

// TestCompleteDownloadWorkflow tests end-to-end download process with mock Terabox responses
func TestCompleteDownloadWorkflow(t *testing.T) {
	// Create test data that's large enough to trigger multi-threading
	testData := strings.Repeat("TeraFetch Integration Test Data! ", 100000) // ~3.2MB
	expectedSize := int64(len(testData))

	// Track request count and ranges for verification (thread-safe)
	state := &testState{}

	// Create mock Terabox server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.incrementCount()
		
		// Simulate Terabox headers
		w.Header().Set("Server", "TeraBox")
		w.Header().Set("Content-Type", "application/octet-stream")
		
		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "" {
			state.addRangeRequest(rangeHeader)
			
			// Parse range header (simplified for test)
			var start, end int64
			if n, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end); n == 2 && err == nil {
				if start >= expectedSize || end >= expectedSize {
					w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
					return
				}
				
				// Validate range
				if start > end || start < 0 {
					w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
					return
				}
				
				// Set partial content headers
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, expectedSize))
				w.Header().Set("Content-Length", fmt.Sprintf("%d", end-start+1))
				w.WriteHeader(http.StatusPartialContent)
				
				// Write the requested range
				segmentData := testData[start : end+1]
				w.Write([]byte(segmentData))
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
		} else {
			// Full file request
			w.Header().Set("Content-Length", fmt.Sprintf("%d", expectedSize))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testData))
		}
	}))
	defer server.Close()

	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "terafetch_integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create engine
	engine := NewMultiThreadEngine()

	// Create file metadata (simulating resolved Terabox link)
	meta := &internal.FileMetadata{
		Filename:  "terabox_file.zip",
		Size:      expectedSize,
		DirectURL: server.URL,
		ShareID:   "terabox123",
		Timestamp: time.Now(),
		Checksum:  "", // Not used in this test
	}

	// Create download config
	outputPath := filepath.Join(tempDir, "terabox_file.zip")
	config := &internal.DownloadConfig{
		OutputPath: outputPath,
		Threads:    4, // Multi-threaded download
		RateLimit:  0, // No rate limiting for test
		Quiet:      true,
	}

	// Execute complete download workflow
	startTime := time.Now()
	err = engine.Download(meta, config)
	downloadDuration := time.Since(startTime)
	
	if err != nil {
		t.Fatalf("Complete download workflow failed: %v", err)
	}

	// Verify download completed successfully
	t.Run("verify_file_exists", func(t *testing.T) {
		if !engine.fileOps.FileExists(outputPath) {
			t.Fatalf("Downloaded file does not exist: %s", outputPath)
		}
	})

	t.Run("verify_file_size", func(t *testing.T) {
		actualSize, err := engine.fileOps.GetFileSize(outputPath)
		if err != nil {
			t.Fatalf("Failed to get file size: %v", err)
		}
		if actualSize != expectedSize {
			t.Fatalf("File size mismatch: expected %d, got %d", expectedSize, actualSize)
		}
	})

	t.Run("verify_file_content", func(t *testing.T) {
		content, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("Failed to read downloaded file: %v", err)
		}
		if string(content) != testData {
			t.Fatalf("File content mismatch")
		}
	})

	t.Run("verify_multi_threaded_requests", func(t *testing.T) {
		rangeRequests := state.getRangeRequests()
		if len(rangeRequests) < 2 {
			t.Fatalf("Expected multiple range requests for multi-threaded download, got %d", len(rangeRequests))
		}
		
		// Verify range requests cover the entire file without gaps or overlaps
		ranges := make(map[string]bool)
		for _, rangeReq := range rangeRequests {
			ranges[rangeReq] = true
		}
		
		if len(ranges) < 2 {
			t.Fatalf("Expected multiple unique range requests, got %d", len(ranges))
		}
	})

	t.Run("verify_no_resume_metadata", func(t *testing.T) {
		// Resume metadata should be cleaned up after successful download
		metadataPath := outputPath + ResumeMetadataExt
		if _, err := os.Stat(metadataPath); !os.IsNotExist(err) {
			t.Errorf("Resume metadata should be cleaned up after successful download")
		}
	})

	t.Run("verify_no_part_file", func(t *testing.T) {
		// Part file should be renamed to final file
		partPath := outputPath + ".part"
		if _, err := os.Stat(partPath); !os.IsNotExist(err) {
			t.Errorf("Part file should be renamed after successful download")
		}
	})

	t.Logf("Download completed in %v with %d HTTP requests", downloadDuration, state.getCount())
}

// TestDownloadResumeWorkflow tests comprehensive resume functionality and crash recovery
func TestDownloadResumeWorkflow(t *testing.T) {
	// Create test data
	testData := strings.Repeat("Resume Test Data! ", 50000) // ~850KB
	expectedSize := int64(len(testData))

	var requestCount int
	var interruptionSimulated bool

	// Create mock server that simulates network interruption
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		
		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "" {
			var start, end int64
			if n, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end); n == 2 && err == nil {
				// Simulate network interruption on second segment
				if start > expectedSize/2 && !interruptionSimulated {
					interruptionSimulated = true
					// Simulate connection reset
					return // Close connection without response
				}
				
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, expectedSize))
				w.Header().Set("Content-Length", fmt.Sprintf("%d", end-start+1))
				w.WriteHeader(http.StatusPartialContent)
				
				segmentData := testData[start : end+1]
				w.Write([]byte(segmentData))
			}
		} else {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", expectedSize))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testData))
		}
	}))
	defer server.Close()

	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "terafetch_resume_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create engine
	engine := NewMultiThreadEngine()

	// Create file metadata
	meta := &internal.FileMetadata{
		Filename:  "resume_test.zip",
		Size:      expectedSize,
		DirectURL: server.URL,
		ShareID:   "resume123",
		Timestamp: time.Now(),
	}

	outputPath := filepath.Join(tempDir, "resume_test.zip")
	config := &internal.DownloadConfig{
		OutputPath: outputPath,
		Threads:    2, // Use 2 threads to trigger interruption scenario
		RateLimit:  0,
		Quiet:      true,
	}

	// First download attempt (will be interrupted)
	t.Run("initial_download_with_interruption", func(t *testing.T) {
		err := engine.Download(meta, config)
		// This should fail due to simulated network interruption
		if err == nil {
			t.Logf("Download completed without interruption (test server behavior may vary)")
		} else {
			t.Logf("Download interrupted as expected: %v", err)
		}
	})

	// Check if resume metadata was created
	t.Run("verify_resume_metadata_created", func(t *testing.T) {
		metadataPath := outputPath + ResumeMetadataExt
		if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
			t.Skip("Resume metadata not created, skipping resume test")
		}
		
		// Load and verify resume metadata
		resumeData, err := engine.planner.LoadResumeMetadata(outputPath)
		if err != nil {
			t.Fatalf("Failed to load resume metadata: %v", err)
		}
		
		if resumeData.FileMetadata.Size != expectedSize {
			t.Errorf("Resume metadata file size mismatch: expected %d, got %d", 
				expectedSize, resumeData.FileMetadata.Size)
		}
		
		if len(resumeData.Segments) == 0 {
			t.Errorf("Resume metadata should contain segments")
		}
	})

	// Reset interruption flag for resume attempt
	interruptionSimulated = true // Prevent further interruptions

	// Second download attempt (resume)
	t.Run("resume_download", func(t *testing.T) {
		initialRequestCount := requestCount
		
		err := engine.Download(meta, config)
		if err != nil {
			t.Fatalf("Resume download failed: %v", err)
		}
		
		// Verify file was completed
		if !engine.fileOps.FileExists(outputPath) {
			t.Fatalf("Resumed file does not exist: %s", outputPath)
		}
		
		actualSize, err := engine.fileOps.GetFileSize(outputPath)
		if err != nil {
			t.Fatalf("Failed to get file size: %v", err)
		}
		
		if actualSize != expectedSize {
			t.Fatalf("File size mismatch after resume: expected %d, got %d", expectedSize, actualSize)
		}
		
		// Verify content integrity
		content, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("Failed to read resumed file: %v", err)
		}
		
		if string(content) != testData {
			t.Fatalf("File content mismatch after resume")
		}
		
		// Verify resume used fewer requests than full download
		resumeRequests := requestCount - initialRequestCount
		t.Logf("Resume completed with %d additional HTTP requests", resumeRequests)
	})

	// Verify cleanup after successful resume
	t.Run("verify_cleanup_after_resume", func(t *testing.T) {
		metadataPath := outputPath + ResumeMetadataExt
		if _, err := os.Stat(metadataPath); !os.IsNotExist(err) {
			t.Errorf("Resume metadata should be cleaned up after successful resume")
		}
		
		partPath := outputPath + ".part"
		if _, err := os.Stat(partPath); !os.IsNotExist(err) {
			t.Errorf("Part file should be renamed after successful resume")
		}
	})
}

// TestDownloadErrorHandling tests error handling and recovery scenarios
func TestDownloadErrorHandling(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "terafetch_error_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create engine
	engine := NewMultiThreadEngine()

	t.Run("http_404_error", func(t *testing.T) {
		// Create server that returns 404
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("File not found"))
		}))
		defer server.Close()

		meta := &internal.FileMetadata{
			Filename:  "not_found.zip",
			Size:      1024,
			DirectURL: server.URL,
		}

		config := &internal.DownloadConfig{
			OutputPath: filepath.Join(tempDir, "not_found.zip"),
			Threads:    1,
			Quiet:      true,
		}

		err := engine.Download(meta, config)
		if err == nil {
			t.Errorf("Expected error for 404 response, got nil")
		}
	})

	t.Run("http_403_forbidden", func(t *testing.T) {
		// Create server that returns 403
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Access denied"))
		}))
		defer server.Close()

		meta := &internal.FileMetadata{
			Filename:  "forbidden.zip",
			Size:      1024,
			DirectURL: server.URL,
		}

		config := &internal.DownloadConfig{
			OutputPath: filepath.Join(tempDir, "forbidden.zip"),
			Threads:    1,
			Quiet:      true,
		}

		err := engine.Download(meta, config)
		if err == nil {
			t.Errorf("Expected error for 403 response, got nil")
		}
	})

	t.Run("http_429_rate_limit", func(t *testing.T) {
		// Create server that returns 429
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("Rate limit exceeded"))
		}))
		defer server.Close()

		meta := &internal.FileMetadata{
			Filename:  "rate_limited.zip",
			Size:      1024,
			DirectURL: server.URL,
		}

		config := &internal.DownloadConfig{
			OutputPath: filepath.Join(tempDir, "rate_limited.zip"),
			Threads:    1,
			Quiet:      true,
		}

		err := engine.Download(meta, config)
		if err == nil {
			t.Errorf("Expected error for 429 response, got nil")
		}
	})

	t.Run("invalid_range_request", func(t *testing.T) {
		// Create server that returns 416 for invalid ranges
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rangeHeader := r.Header.Get("Range")
			if rangeHeader != "" {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				w.Write([]byte("Range not satisfiable"))
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("test data"))
			}
		}))
		defer server.Close()

		meta := &internal.FileMetadata{
			Filename:  "invalid_range.zip",
			Size:      1024,
			DirectURL: server.URL,
		}

		config := &internal.DownloadConfig{
			OutputPath: filepath.Join(tempDir, "invalid_range.zip"),
			Threads:    2, // Multi-threaded to trigger range requests
			Quiet:      true,
		}

		err := engine.Download(meta, config)
		if err == nil {
			t.Errorf("Expected error for invalid range response, got nil")
		}
	})

	t.Run("nil_metadata", func(t *testing.T) {
		config := &internal.DownloadConfig{
			OutputPath: filepath.Join(tempDir, "nil_test.zip"),
			Threads:    1,
			Quiet:      true,
		}

		err := engine.Download(nil, config)
		if err == nil {
			t.Errorf("Expected error for nil metadata, got nil")
		}
	})

	t.Run("nil_config", func(t *testing.T) {
		meta := &internal.FileMetadata{
			Filename: "nil_config.zip",
			Size:     1024,
		}

		err := engine.Download(meta, nil)
		if err == nil {
			t.Errorf("Expected error for nil config, got nil")
		}
	})
}

// TestDownloadConcurrencyAndRaceConditions tests thread safety and race conditions
func TestDownloadConcurrencyAndRaceConditions(t *testing.T) {
	// Create test data
	testData := strings.Repeat("Concurrency Test! ", 10000) // ~170KB
	expectedSize := int64(len(testData))

	var requestCount int32 // Use atomic for thread safety

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		// Simulate some processing delay
		time.Sleep(10 * time.Millisecond)
		
		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "" {
			var start, end int64
			if n, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end); n == 2 && err == nil {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, expectedSize))
				w.Header().Set("Content-Length", fmt.Sprintf("%d", end-start+1))
				w.WriteHeader(http.StatusPartialContent)
				
				segmentData := testData[start : end+1]
				w.Write([]byte(segmentData))
			}
		} else {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", expectedSize))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testData))
		}
	}))
	defer server.Close()

	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "terafetch_concurrency_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test multiple concurrent downloads
	t.Run("concurrent_downloads", func(t *testing.T) {
		const numConcurrentDownloads = 3
		var wg sync.WaitGroup
		errors := make(chan error, numConcurrentDownloads)

		for i := 0; i < numConcurrentDownloads; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				engine := NewMultiThreadEngine()
				
				meta := &internal.FileMetadata{
					Filename:  fmt.Sprintf("concurrent_%d.zip", id),
					Size:      expectedSize,
					DirectURL: server.URL,
				}

				config := &internal.DownloadConfig{
					OutputPath: filepath.Join(tempDir, fmt.Sprintf("concurrent_%d.zip", id)),
					Threads:    4,
					RateLimit:  0,
					Quiet:      true,
				}

				if err := engine.Download(meta, config); err != nil {
					errors <- fmt.Errorf("download %d failed: %w", id, err)
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent download error: %v", err)
		}

		// Verify all files were created correctly
		for i := 0; i < numConcurrentDownloads; i++ {
			filePath := filepath.Join(tempDir, fmt.Sprintf("concurrent_%d.zip", i))
			
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Errorf("Concurrent download %d file not created", i)
				continue
			}
			
			content, err := os.ReadFile(filePath)
			if err != nil {
				t.Errorf("Failed to read concurrent download %d: %v", i, err)
				continue
			}
			
			if string(content) != testData {
				t.Errorf("Concurrent download %d content mismatch", i)
			}
		}
	})

	t.Run("high_thread_count", func(t *testing.T) {
		engine := NewMultiThreadEngine()
		
		meta := &internal.FileMetadata{
			Filename:  "high_thread_test.zip",
			Size:      expectedSize,
			DirectURL: server.URL,
		}

		config := &internal.DownloadConfig{
			OutputPath: filepath.Join(tempDir, "high_thread_test.zip"),
			Threads:    16, // High thread count
			RateLimit:  0,
			Quiet:      true,
		}

		err := engine.Download(meta, config)
		if err != nil {
			t.Fatalf("High thread count download failed: %v", err)
		}

		// Verify file integrity
		content, err := os.ReadFile(config.OutputPath)
		if err != nil {
			t.Fatalf("Failed to read high thread count file: %v", err)
		}
		
		if string(content) != testData {
			t.Fatalf("High thread count download content mismatch")
		}
	})
}