package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"terafetch/internal"
)

const (
	// MinSegmentSize is the minimum size for a download segment (1MB)
	MinSegmentSize = 1024 * 1024
	// MaxThreads is the maximum number of download threads allowed
	MaxThreads = 32
	// ResumeMetadataExt is the file extension for resume metadata files
	ResumeMetadataExt = ".terafetch.json"
)

// DownloadPlanner handles download strategy planning and segmentation
type DownloadPlanner struct {
	minSegmentSize int64
	maxThreads     int
}

// NewDownloadPlanner creates a new instance of DownloadPlanner
func NewDownloadPlanner() *DownloadPlanner {
	return &DownloadPlanner{
		minSegmentSize: MinSegmentSize,
		maxThreads:     MaxThreads,
	}
}

// PlanDownload creates a download plan with optimal segmentation
func (p *DownloadPlanner) PlanDownload(meta *internal.FileMetadata, config *internal.DownloadConfig) ([]internal.SegmentInfo, error) {
	if meta == nil {
		return nil, fmt.Errorf("file metadata cannot be nil")
	}
	if config == nil {
		return nil, fmt.Errorf("download config cannot be nil")
	}

	// Check if we should resume an existing download
	if config.ResumeData != nil {
		return p.planResumeDownload(config.ResumeData, meta)
	}

	// Determine optimal thread count based on file size and configuration
	optimalThreads := p.determineOptimalThreads(meta.Size, config.Threads)
	
	// Calculate segments based on file size and thread count
	segments := p.CalculateSegments(meta.Size, optimalThreads)
	
	return segments, nil
}

// CalculateSegments determines optimal segment sizes for multi-threaded downloads
func (p *DownloadPlanner) CalculateSegments(fileSize int64, threadCount int) []internal.SegmentInfo {
	// Validate inputs
	if fileSize <= 0 {
		return []internal.SegmentInfo{}
	}
	
	if threadCount <= 0 {
		threadCount = 1
	}
	if threadCount > p.maxThreads {
		threadCount = p.maxThreads
	}

	// For small files, use single-threaded download
	if fileSize < p.minSegmentSize {
		return []internal.SegmentInfo{
			{
				Index:     0,
				Start:     0,
				End:       fileSize - 1,
				Completed: false,
				Retries:   0,
			},
		}
	}

	// Calculate segment size
	segmentSize := fileSize / int64(threadCount)
	
	// Ensure each segment is at least MinSegmentSize
	if segmentSize < p.minSegmentSize {
		// Reduce thread count to maintain minimum segment size
		threadCount = int(fileSize / p.minSegmentSize)
		if threadCount == 0 {
			threadCount = 1
		}
		segmentSize = fileSize / int64(threadCount)
	}

	segments := make([]internal.SegmentInfo, 0, threadCount)
	
	for i := 0; i < threadCount; i++ {
		start := int64(i) * segmentSize
		end := start + segmentSize - 1
		
		// Last segment gets any remaining bytes
		if i == threadCount-1 {
			end = fileSize - 1
		}
		
		segments = append(segments, internal.SegmentInfo{
			Index:     i,
			Start:     start,
			End:       end,
			Completed: false,
			Retries:   0,
		})
	}
	
	return segments
}

// determineOptimalThreads calculates the optimal number of threads based on file size
func (p *DownloadPlanner) determineOptimalThreads(fileSize int64, requestedThreads int) int {
	// Start with requested threads
	optimalThreads := requestedThreads
	
	// Validate thread count bounds
	if optimalThreads <= 0 {
		optimalThreads = 1
	}
	if optimalThreads > p.maxThreads {
		optimalThreads = p.maxThreads
	}
	
	// For small files, limit threads to maintain minimum segment size
	maxPossibleThreads := int(fileSize / p.minSegmentSize)
	if maxPossibleThreads == 0 {
		maxPossibleThreads = 1
	}
	
	if optimalThreads > maxPossibleThreads {
		optimalThreads = maxPossibleThreads
	}
	
	return optimalThreads
}

// planResumeDownload creates a plan for resuming an interrupted download
func (p *DownloadPlanner) planResumeDownload(resumeData *internal.ResumeMetadata, currentMeta *internal.FileMetadata) ([]internal.SegmentInfo, error) {
	if resumeData.FileMetadata == nil {
		return nil, fmt.Errorf("resume metadata missing file information")
	}
	
	// Verify file metadata matches
	if resumeData.FileMetadata.Size != currentMeta.Size {
		return nil, fmt.Errorf("file size mismatch: expected %d, got %d", resumeData.FileMetadata.Size, currentMeta.Size)
	}
	
	if resumeData.FileMetadata.Filename != currentMeta.Filename {
		return nil, fmt.Errorf("filename mismatch: expected %s, got %s", resumeData.FileMetadata.Filename, currentMeta.Filename)
	}
	
	// Return existing segments for resume
	return resumeData.Segments, nil
}

// SaveResumeMetadata saves download progress metadata to disk
func (p *DownloadPlanner) SaveResumeMetadata(outputPath string, meta *internal.FileMetadata, segments []internal.SegmentInfo) error {
	resumeData := &internal.ResumeMetadata{
		FileMetadata: meta,
		Segments:     segments,
		CreatedAt:    time.Now(),
		LastUpdate:   time.Now(),
	}
	
	metadataPath := outputPath + ResumeMetadataExt
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}
	
	// Marshal to JSON
	data, err := json.MarshalIndent(resumeData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal resume metadata: %w", err)
	}
	
	// Write to file
	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write resume metadata: %w", err)
	}
	
	return nil
}

// LoadResumeMetadata loads download progress metadata from disk
func (p *DownloadPlanner) LoadResumeMetadata(outputPath string) (*internal.ResumeMetadata, error) {
	metadataPath := outputPath + ResumeMetadataExt
	
	// Check if metadata file exists
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("resume metadata not found: %s", metadataPath)
	}
	
	// Read metadata file
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read resume metadata: %w", err)
	}
	
	// Unmarshal JSON
	var resumeData internal.ResumeMetadata
	if err := json.Unmarshal(data, &resumeData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resume metadata: %w", err)
	}
	
	return &resumeData, nil
}

// UpdateSegmentProgress updates the progress of a specific segment
func (p *DownloadPlanner) UpdateSegmentProgress(outputPath string, segmentIndex int, completed bool) error {
	// Load existing metadata
	resumeData, err := p.LoadResumeMetadata(outputPath)
	if err != nil {
		return fmt.Errorf("failed to load resume metadata: %w", err)
	}
	
	// Validate segment index
	if segmentIndex < 0 || segmentIndex >= len(resumeData.Segments) {
		return fmt.Errorf("invalid segment index: %d", segmentIndex)
	}
	
	// Update segment
	resumeData.Segments[segmentIndex].Completed = completed
	resumeData.LastUpdate = time.Now()
	
	// Save updated metadata
	return p.saveResumeMetadataStruct(outputPath, resumeData)
}

// IncrementSegmentRetries increments the retry count for a specific segment
func (p *DownloadPlanner) IncrementSegmentRetries(outputPath string, segmentIndex int) error {
	// Load existing metadata
	resumeData, err := p.LoadResumeMetadata(outputPath)
	if err != nil {
		return fmt.Errorf("failed to load resume metadata: %w", err)
	}
	
	// Validate segment index
	if segmentIndex < 0 || segmentIndex >= len(resumeData.Segments) {
		return fmt.Errorf("invalid segment index: %d", segmentIndex)
	}
	
	// Increment retries
	resumeData.Segments[segmentIndex].Retries++
	resumeData.LastUpdate = time.Now()
	
	// Save updated metadata
	return p.saveResumeMetadataStruct(outputPath, resumeData)
}

// IsDownloadComplete checks if all segments are completed
func (p *DownloadPlanner) IsDownloadComplete(segments []internal.SegmentInfo) bool {
	for _, segment := range segments {
		if !segment.Completed {
			return false
		}
	}
	return len(segments) > 0
}

// CleanupResumeMetadata removes the resume metadata file after successful download
func (p *DownloadPlanner) CleanupResumeMetadata(outputPath string) error {
	metadataPath := outputPath + ResumeMetadataExt
	
	if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to cleanup resume metadata: %w", err)
	}
	
	return nil
}

// DetectResumableDownload checks if a download can be resumed
func (p *DownloadPlanner) DetectResumableDownload(outputPath string) (*internal.ResumeMetadata, error) {
	metadataPath := outputPath + ResumeMetadataExt
	partPath := outputPath + ".part"
	
	// Check if both metadata and part file exist
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return nil, nil // No resume data available
	}
	
	if _, err := os.Stat(partPath); os.IsNotExist(err) {
		// Metadata exists but no part file - cleanup stale metadata
		os.Remove(metadataPath)
		return nil, nil
	}
	
	// Load and validate resume metadata
	resumeData, err := p.LoadResumeMetadata(outputPath)
	if err != nil {
		// Invalid metadata - cleanup and start fresh
		os.Remove(metadataPath)
		os.Remove(partPath)
		return nil, fmt.Errorf("invalid resume metadata, cleaned up: %w", err)
	}
	
	// Validate part file against metadata
	partInfo, err := os.Stat(partPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat part file: %w", err)
	}
	
	expectedSize := resumeData.FileMetadata.Size
	if partInfo.Size() > expectedSize {
		// Part file is larger than expected - cleanup and start fresh
		os.Remove(metadataPath)
		os.Remove(partPath)
		return nil, fmt.Errorf("part file size exceeds expected size, cleaned up")
	}
	
	return resumeData, nil
}

// ValidateResumeCompatibility checks if resume data is compatible with current download
func (p *DownloadPlanner) ValidateResumeCompatibility(resumeData *internal.ResumeMetadata, currentMeta *internal.FileMetadata) error {
	if resumeData.FileMetadata == nil {
		return fmt.Errorf("resume metadata missing file information")
	}
	
	// Check file size compatibility
	if resumeData.FileMetadata.Size != currentMeta.Size {
		return fmt.Errorf("file size changed: resume=%d, current=%d", 
			resumeData.FileMetadata.Size, currentMeta.Size)
	}
	
	// Check filename compatibility (allow some flexibility)
	if resumeData.FileMetadata.Filename != currentMeta.Filename {
		// Log warning but don't fail - filename might have been updated
		fmt.Printf("Warning: filename changed from %s to %s\n", 
			resumeData.FileMetadata.Filename, currentMeta.Filename)
	}
	
	// Check if resume data is not too old (7 days)
	maxAge := 7 * 24 * time.Hour
	if time.Since(resumeData.LastUpdate) > maxAge {
		return fmt.Errorf("resume data is too old (last update: %s)", resumeData.LastUpdate.Format(time.RFC3339))
	}
	
	return nil
}

// RecoverFromNetworkInterruption attempts to recover from network failures
func (p *DownloadPlanner) RecoverFromNetworkInterruption(outputPath string, segmentIndex int, err error) error {
	// Increment retry count for the failed segment
	if retryErr := p.IncrementSegmentRetries(outputPath, segmentIndex); retryErr != nil {
		return fmt.Errorf("failed to update retry count: %w", retryErr)
	}
	
	// Load current retry count
	resumeData, loadErr := p.LoadResumeMetadata(outputPath)
	if loadErr != nil {
		return fmt.Errorf("failed to load resume data for recovery: %w", loadErr)
	}
	
	if segmentIndex >= len(resumeData.Segments) {
		return fmt.Errorf("invalid segment index for recovery: %d", segmentIndex)
	}
	
	segment := resumeData.Segments[segmentIndex]
	maxRetries := 5 // Maximum retries per segment
	
	if segment.Retries >= maxRetries {
		return fmt.Errorf("segment %d exceeded maximum retries (%d): %w", segmentIndex, maxRetries, err)
	}
	
	// Calculate backoff delay based on retry count
	backoffDelay := time.Duration(segment.Retries*segment.Retries) * time.Second
	if backoffDelay > 30*time.Second {
		backoffDelay = 30 * time.Second
	}
	
	fmt.Printf("Network interruption on segment %d (retry %d/%d), backing off for %v\n", 
		segmentIndex, segment.Retries, maxRetries, backoffDelay)
	
	time.Sleep(backoffDelay)
	return nil
}

// GetIncompleteSegments returns segments that need to be downloaded
func (p *DownloadPlanner) GetIncompleteSegments(segments []internal.SegmentInfo) []internal.SegmentInfo {
	var incomplete []internal.SegmentInfo
	for _, segment := range segments {
		if !segment.Completed {
			incomplete = append(incomplete, segment)
		}
	}
	return incomplete
}

// CalculateResumeProgress returns the percentage of download completed
func (p *DownloadPlanner) CalculateResumeProgress(segments []internal.SegmentInfo) float64 {
	if len(segments) == 0 {
		return 0.0
	}
	
	var totalBytes, completedBytes int64
	for _, segment := range segments {
		segmentSize := segment.End - segment.Start + 1
		totalBytes += segmentSize
		if segment.Completed {
			completedBytes += segmentSize
		}
	}
	
	if totalBytes == 0 {
		return 0.0
	}
	
	return float64(completedBytes) / float64(totalBytes) * 100.0
}

// saveResumeMetadataStruct saves a ResumeMetadata struct to disk
func (p *DownloadPlanner) saveResumeMetadataStruct(outputPath string, resumeData *internal.ResumeMetadata) error {
	metadataPath := outputPath + ResumeMetadataExt
	
	// Marshal to JSON
	data, err := json.MarshalIndent(resumeData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal resume metadata: %w", err)
	}
	
	// Write to file
	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write resume metadata: %w", err)
	}
	
	return nil
}