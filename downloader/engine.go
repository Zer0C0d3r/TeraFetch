package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"terafetch/internal"
	"terafetch/utils"
)

// DownloadJob represents a segment download job
type DownloadJob struct {
	Segment    internal.SegmentInfo
	FileURL    string
	OutputPath string
	PartPath   string
}

// DownloadResult represents the result of a segment download
type DownloadResult struct {
	SegmentIndex int
	BytesWritten int64
	Error        error
	Completed    bool
}

// WorkerPool manages concurrent download workers
type WorkerPool struct {
	workers     int
	jobs        chan DownloadJob
	results     chan DownloadResult
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	httpClient  *utils.HTTPClient
	rateLimiter internal.RateLimiter
}

// MultiThreadEngine implements the DownloadEngine interface
type MultiThreadEngine struct {
	httpClient *utils.HTTPClient
	planner    *DownloadPlanner
	fileOps    *utils.FileOperations
}

// NewMultiThreadEngine creates a new instance of MultiThreadEngine
func NewMultiThreadEngine() *MultiThreadEngine {
	return &MultiThreadEngine{
		httpClient: utils.NewHTTPClient(),
		planner:    NewDownloadPlanner(),
		fileOps:    utils.NewFileOperations(),
	}
}

// Download starts a new multi-threaded download with automatic resume detection
func (e *MultiThreadEngine) Download(meta *internal.FileMetadata, config *internal.DownloadConfig) error {
	if meta == nil {
		return fmt.Errorf("file metadata cannot be nil")
	}
	if config == nil {
		return fmt.Errorf("download config cannot be nil")
	}

	// Determine output path
	outputPath := config.OutputPath
	if outputPath == "" {
		outputPath = meta.Filename
	}

	// Ensure output directory exists
	if err := e.fileOps.EnsureDir(outputPath); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check for existing resumable download
	resumeData, err := e.planner.DetectResumableDownload(outputPath)
	if err != nil {
		fmt.Printf("Warning: %v\n", err)
		resumeData = nil
	}

	var segments []internal.SegmentInfo
	
	if resumeData != nil {
		// Validate resume compatibility
		if err := e.planner.ValidateResumeCompatibility(resumeData, meta); err != nil {
			fmt.Printf("Resume validation failed: %v, starting fresh download\n", err)
			// Cleanup invalid resume data
			e.planner.CleanupResumeMetadata(outputPath)
			os.Remove(outputPath + ".part")
			resumeData = nil
		} else {
			// Resume existing download
			fmt.Printf("Resuming download from %.1f%% completion\n", 
				e.planner.CalculateResumeProgress(resumeData.Segments))
			segments = resumeData.Segments
			config.ResumeData = resumeData
		}
	}

	if resumeData == nil {
		// Plan new download segments
		segments, err = e.planner.PlanDownload(meta, config)
		if err != nil {
			return fmt.Errorf("failed to plan download: %w", err)
		}
	}

	// Create part file path
	partPath := outputPath + ".part"

	// Create or validate part file
	if resumeData == nil {
		if err := e.fileOps.CreatePartialFile(partPath, meta.Size); err != nil {
			return fmt.Errorf("failed to create part file: %w", err)
		}
	} else {
		if err := e.fileOps.ValidatePartialFile(partPath, meta.Size); err != nil {
			return fmt.Errorf("failed to validate part file: %w", err)
		}
	}

	// Save/update resume metadata
	if err := e.planner.SaveResumeMetadata(outputPath, meta, segments); err != nil {
		return fmt.Errorf("failed to save resume metadata: %w", err)
	}

	// Execute the download with retry logic
	if err := e.executeDownloadWithRetry(meta, segments, outputPath, partPath, config); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Verify file integrity
	if err := e.verifyFileIntegrity(outputPath, meta.Size); err != nil {
		return fmt.Errorf("file integrity verification failed: %w", err)
	}

	// Cleanup resume metadata
	if err := e.planner.CleanupResumeMetadata(outputPath); err != nil {
		// Log warning but don't fail the download
		fmt.Printf("Warning: failed to cleanup resume metadata: %v\n", err)
	}

	return nil
}

// Resume continues an interrupted download
func (e *MultiThreadEngine) Resume(partialPath string, config *internal.DownloadConfig) error {
	if config == nil {
		return fmt.Errorf("download config cannot be nil")
	}

	// Load resume metadata
	resumeData, err := e.planner.LoadResumeMetadata(partialPath)
	if err != nil {
		return fmt.Errorf("failed to load resume metadata: %w", err)
	}

	// Update config with resume data
	config.ResumeData = resumeData

	// Continue download with existing metadata
	return e.Download(resumeData.FileMetadata, config)
}

// executeDownloadWithRetry performs download with automatic retry and recovery
func (e *MultiThreadEngine) executeDownloadWithRetry(meta *internal.FileMetadata, segments []internal.SegmentInfo, outputPath, partPath string, config *internal.DownloadConfig) error {
	maxGlobalRetries := 3
	
	for attempt := 0; attempt < maxGlobalRetries; attempt++ {
		err := e.executeDownload(meta, segments, outputPath, partPath, config)
		if err == nil {
			return nil // Success
		}
		
		// Check if error is recoverable
		if !e.isRecoverableError(err) {
			return err // Non-recoverable error
		}
		
		fmt.Printf("Download attempt %d failed: %v\n", attempt+1, err)
		
		if attempt < maxGlobalRetries-1 {
			// Reload segments to get current state
			resumeData, loadErr := e.planner.LoadResumeMetadata(outputPath)
			if loadErr != nil {
				fmt.Printf("Warning: failed to reload resume data: %v\n", loadErr)
			} else {
				segments = resumeData.Segments
			}
			
			// Wait before retry with exponential backoff
			backoffDelay := time.Duration(1<<uint(attempt)) * time.Second
			fmt.Printf("Retrying in %v...\n", backoffDelay)
			time.Sleep(backoffDelay)
		}
	}
	
	return fmt.Errorf("download failed after %d attempts", maxGlobalRetries)
}

// executeDownload performs the actual multi-threaded download
func (e *MultiThreadEngine) executeDownload(meta *internal.FileMetadata, segments []internal.SegmentInfo, outputPath, partPath string, config *internal.DownloadConfig) error {
	// Create or open part file
	partFile, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create part file: %w", err)
	}
	defer partFile.Close()

	// Truncate or extend file to expected size
	if err := partFile.Truncate(meta.Size); err != nil {
		return fmt.Errorf("failed to set part file size: %w", err)
	}

	// Create worker pool
	pool := e.createWorkerPool(config.Threads, config.RateLimit)
	defer pool.shutdown()

	// Start progress tracking
	progressTracker := utils.NewProgressTracker(meta.Size, config.Quiet)
	defer func() {
		summary := progressTracker.Finish()
		if summary != nil {
			summary.Filename = outputPath
		}
	}()

	// Track total progress
	var totalProgress int64
	var progressMutex sync.Mutex

	// Start workers
	pool.start()

	// Submit jobs for incomplete segments
	go func() {
		defer close(pool.jobs)
		for _, segment := range segments {
			if !segment.Completed {
				job := DownloadJob{
					Segment:    segment,
					FileURL:    meta.DirectURL,
					OutputPath: outputPath,
					PartPath:   partPath,
				}
				select {
				case pool.jobs <- job:
				case <-pool.ctx.Done():
					return
				}
			} else {
				// Add completed segment size to progress
				segmentSize := segment.End - segment.Start + 1
				atomic.AddInt64(&totalProgress, segmentSize)
				progressTracker.Update(atomic.LoadInt64(&totalProgress))
			}
		}
	}()

	// Process results
	completedSegments := 0
	expectedSegments := len(segments)
	
	// Count already completed segments
	for _, segment := range segments {
		if segment.Completed {
			completedSegments++
		}
	}

	for result := range pool.results {
		if result.Error != nil {
			pool.cancel()
			return fmt.Errorf("segment %d download failed: %w", result.SegmentIndex, result.Error)
		}

		if result.Completed {
			// Update segment progress in metadata
			if err := e.planner.UpdateSegmentProgress(outputPath, result.SegmentIndex, true); err != nil {
				fmt.Printf("Warning: failed to update segment progress: %v\n", err)
			}
			completedSegments++
		}

		// Update progress
		progressMutex.Lock()
		totalProgress += result.BytesWritten
		progressTracker.Update(totalProgress)
		progressMutex.Unlock()

		// Check if all segments are complete
		if completedSegments >= expectedSegments {
			break
		}
	}

	// Perform atomic rename from .part to final file
	if err := e.fileOps.AtomicRename(partPath, outputPath); err != nil {
		return fmt.Errorf("failed to rename part file to final file: %w", err)
	}

	return nil
}

// createWorkerPool creates a new worker pool for downloads
func (e *MultiThreadEngine) createWorkerPool(workers int, rateLimit int64) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	
	var rateLimiter internal.RateLimiter
	if rateLimit > 0 {
		rateLimiter = utils.NewDistributedRateLimiter(rateLimit, workers)
	}

	return &WorkerPool{
		workers:     workers,
		jobs:        make(chan DownloadJob, workers*2),
		results:     make(chan DownloadResult, workers*2),
		ctx:         ctx,
		cancel:      cancel,
		httpClient:  e.httpClient,
		rateLimiter: rateLimiter,
	}
}

// start begins the worker pool execution
func (wp *WorkerPool) start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}

	// Start result collector
	go func() {
		wp.wg.Wait()
		close(wp.results)
	}()
}

// shutdown gracefully shuts down the worker pool
func (wp *WorkerPool) shutdown() {
	wp.cancel()
	wp.wg.Wait()
}

// worker processes download jobs
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	// Register this worker thread with the rate limiter
	if wp.rateLimiter != nil {
		if limiter, ok := wp.rateLimiter.(*utils.TokenBucketLimiter); ok {
			limiter.RegisterThread()
			defer limiter.UnregisterThread()
		}
	}

	for {
		select {
		case job, ok := <-wp.jobs:
			if !ok {
				return
			}
			result := wp.processJob(job)
			select {
			case wp.results <- result:
			case <-wp.ctx.Done():
				return
			}
		case <-wp.ctx.Done():
			return
		}
	}
}

// processJob downloads a single segment with retry logic
func (wp *WorkerPool) processJob(job DownloadJob) DownloadResult {
	result := DownloadResult{
		SegmentIndex: job.Segment.Index,
		BytesWritten: 0,
		Error:        nil,
		Completed:    false,
	}

	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := wp.downloadSegment(job, &result)
		if err == nil {
			result.Completed = true
			return result
		}
		
		// Check if error is recoverable
		if !wp.isNetworkError(err) || attempt == maxRetries-1 {
			result.Error = err
			return result
		}
		
		// Wait before retry with exponential backoff
		backoffDelay := time.Duration(1<<uint(attempt)) * time.Second
		select {
		case <-time.After(backoffDelay):
			// Continue to retry
		case <-wp.ctx.Done():
			result.Error = wp.ctx.Err()
			return result
		}
	}
	
	return result
}

// downloadSegment performs the actual segment download
func (wp *WorkerPool) downloadSegment(job DownloadJob, result *DownloadResult) error {
	// Open part file for writing at specific offset
	file, err := os.OpenFile(job.PartPath, os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open part file: %w", err)
	}
	defer file.Close()

	// Seek to segment start position
	if _, err := file.Seek(job.Segment.Start, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to segment position: %w", err)
	}

	// Create HTTP request with Range header
	rangeHeader := fmt.Sprintf("bytes=%d-%d", job.Segment.Start, job.Segment.End)

	// Execute request with retry-aware HTTP client
	resp, err := wp.httpClient.GetWithContext(wp.ctx, job.FileURL, map[string]string{
		"Range":      rangeHeader,
		"User-Agent": wp.httpClient.GetCurrentUserAgent(),
	})
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Verify partial content response
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		// Handle specific HTTP errors
		switch resp.StatusCode {
		case http.StatusRequestedRangeNotSatisfiable:
			return fmt.Errorf("range not satisfiable: segment may be invalid")
		case http.StatusTooManyRequests:
			return internal.NewRateLimitError(60) // Suggest 60 second retry
		case http.StatusForbidden:
			return internal.NewTeraboxError(403, "Access forbidden", internal.ErrPermissionDenied)
		default:
			return fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
		}
	}

	// Copy data with rate limiting and progress tracking
	bytesWritten, err := wp.copyWithRateLimit(file, resp.Body, job.Segment.End-job.Segment.Start+1)
	if err != nil {
		return fmt.Errorf("failed to copy segment data: %w", err)
	}

	result.BytesWritten = bytesWritten
	return nil
}

// isNetworkError checks if an error is a recoverable network error
func (wp *WorkerPool) isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	networkErrors := []string{
		"connection reset",
		"connection refused", 
		"timeout",
		"temporary failure",
		"network is unreachable",
		"no route to host",
		"broken pipe",
		"context deadline exceeded",
		"EOF",
	}
	
	for _, pattern := range networkErrors {
		if containsErrorSubstring(errStr, pattern) {
			return true
		}
	}
	
	return false
}

// copyWithRateLimit copies data from reader to writer with rate limiting
func (wp *WorkerPool) copyWithRateLimit(dst io.Writer, src io.Reader, maxBytes int64) (int64, error) {
	const bufferSize = 32 * 1024 // 32KB buffer
	buffer := make([]byte, bufferSize)
	var totalWritten int64

	for totalWritten < maxBytes {
		// Calculate how much to read
		toRead := bufferSize
		remaining := maxBytes - totalWritten
		if int64(toRead) > remaining {
			toRead = int(remaining)
		}

		// Read from source
		n, err := src.Read(buffer[:toRead])
		if n > 0 {
			// Apply rate limiting if configured
			if wp.rateLimiter != nil {
				if err := wp.rateLimiter.Wait(wp.ctx, n); err != nil {
					return totalWritten, fmt.Errorf("rate limiting error: %w", err)
				}
			}

			// Write to destination
			written, writeErr := dst.Write(buffer[:n])
			totalWritten += int64(written)

			if writeErr != nil {
				return totalWritten, writeErr
			}

			if written != n {
				return totalWritten, fmt.Errorf("short write: wrote %d, expected %d", written, n)
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return totalWritten, err
		}

		// Check for context cancellation
		select {
		case <-wp.ctx.Done():
			return totalWritten, wp.ctx.Err()
		default:
		}
	}

	return totalWritten, nil
}

// verifyFileIntegrity checks if the downloaded file matches expected size
func (e *MultiThreadEngine) verifyFileIntegrity(filePath string, expectedSize int64) error {
	actualSize, err := e.fileOps.GetFileSize(filePath)
	if err != nil {
		return fmt.Errorf("failed to get file size: %w", err)
	}

	if actualSize != expectedSize {
		return fmt.Errorf("file size mismatch: expected %d bytes, got %d bytes", expectedSize, actualSize)
	}

	return nil
}

// isRecoverableError determines if an error is recoverable through retry
func (e *MultiThreadEngine) isRecoverableError(err error) bool {
	if err == nil {
		return false
	}
	
	// Check for TeraboxError types
	if teraboxErr, ok := err.(*internal.TeraboxError); ok {
		return teraboxErr.IsRetryable()
	}
	
	// Check for common network errors
	errStr := err.Error()
	recoverablePatterns := []string{
		"connection reset",
		"connection refused",
		"timeout",
		"temporary failure",
		"network is unreachable",
		"no route to host",
		"broken pipe",
		"context deadline exceeded",
	}
	
	for _, pattern := range recoverablePatterns {
		if containsError(errStr, pattern) {
			return true
		}
	}
	
	return false
}

// containsError checks if a string contains a substring (case-insensitive)
func containsError(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		    len(s) > len(substr) && 
		    (s[:len(substr)] == substr || 
		     s[len(s)-len(substr):] == substr || 
		     containsErrorSubstring(s, substr)))
}

// containsErrorSubstring performs case-insensitive substring search
func containsErrorSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}