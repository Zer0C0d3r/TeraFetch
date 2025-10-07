package utils

import (
	"fmt"
	"sync"
	"time"

	"github.com/cheggaaa/pb/v3"
)

// ProgressTracker manages download progress display with real-time statistics
type ProgressTracker struct {
	bar       *pb.ProgressBar
	quiet     bool
	startTime time.Time
	total     int64
	current   int64
	mutex     sync.RWMutex
	
	// Statistics tracking
	lastUpdate     time.Time
	lastBytes      int64
	speedSamples   []float64
	maxSamples     int
}

// DownloadSummary contains final download statistics
type DownloadSummary struct {
	TotalBytes    int64
	TotalTime     time.Duration
	AverageSpeed  float64 // bytes per second
	PeakSpeed     float64 // bytes per second
	Filename      string
}

// NewProgressTracker creates a new progress tracker with enhanced statistics
func NewProgressTracker(total int64, quiet bool) *ProgressTracker {
	tracker := &ProgressTracker{
		quiet:        quiet,
		startTime:    time.Now(),
		total:        total,
		current:      0,
		lastUpdate:   time.Now(),
		lastBytes:    0,
		speedSamples: make([]float64, 0),
		maxSamples:   10, // Keep last 10 speed samples for smoothing
	}
	
	if !quiet {
		// Create progress bar with custom template showing speed and ETA
		tmpl := `{{string . "prefix"}}{{counters . }} {{bar . }} {{percent . }} {{speed . }} {{rtime . "ETA %s"}}`
		bar := pb.ProgressBarTemplate(tmpl).Start64(total)
		bar.Set(pb.Bytes, true)
		bar.Set(pb.SIBytesPrefix, true)
		bar.Set("prefix", "Downloading: ")
		tracker.bar = bar
	}
	
	return tracker
}

// Update updates the progress bar with current progress and calculates real-time statistics
func (p *ProgressTracker) Update(current int64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	now := time.Now()
	p.current = current
	
	// Calculate current speed
	if p.bar != nil {
		p.bar.SetCurrent(current)
		
		// Update speed calculation
		timeDiff := now.Sub(p.lastUpdate).Seconds()
		if timeDiff > 0.1 { // Update speed every 100ms to avoid too frequent updates
			bytesDiff := current - p.lastBytes
			currentSpeed := float64(bytesDiff) / timeDiff
			
			// Add to speed samples for smoothing
			p.speedSamples = append(p.speedSamples, currentSpeed)
			if len(p.speedSamples) > p.maxSamples {
				p.speedSamples = p.speedSamples[1:]
			}
			
			// Calculate smoothed speed
			var avgSpeed float64
			for _, speed := range p.speedSamples {
				avgSpeed += speed
			}
			if len(p.speedSamples) > 0 {
				avgSpeed /= float64(len(p.speedSamples))
			}
			
			// Update progress bar with current speed
			p.bar.Set(pb.Static, fmt.Sprintf("%.2f MB/s", avgSpeed/(1024*1024)))
			
			p.lastUpdate = now
			p.lastBytes = current
		}
	}
}

// Finish completes the progress bar and returns download summary
func (p *ProgressTracker) Finish() *DownloadSummary {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	endTime := time.Now()
	totalTime := endTime.Sub(p.startTime)
	
	if p.bar != nil {
		p.bar.Finish()
	}
	
	// Calculate final statistics
	averageSpeed := float64(p.current) / totalTime.Seconds()
	
	// Find peak speed from samples
	var peakSpeed float64
	for _, speed := range p.speedSamples {
		if speed > peakSpeed {
			peakSpeed = speed
		}
	}
	
	summary := &DownloadSummary{
		TotalBytes:   p.current,
		TotalTime:    totalTime,
		AverageSpeed: averageSpeed,
		PeakSpeed:    peakSpeed,
	}
	
	// Display summary if not in quiet mode
	if !p.quiet {
		p.displaySummary(summary)
	}
	
	return summary
}

// displaySummary prints the download summary statistics
func (p *ProgressTracker) displaySummary(summary *DownloadSummary) {
	fmt.Printf("\n")
	fmt.Printf("Download completed successfully!\n")
	fmt.Printf("Total size: %s\n", formatBytes(summary.TotalBytes))
	fmt.Printf("Total time: %v\n", summary.TotalTime.Round(time.Millisecond))
	fmt.Printf("Average speed: %s/s\n", formatBytes(int64(summary.AverageSpeed)))
	if summary.PeakSpeed > 0 {
		fmt.Printf("Peak speed: %s/s\n", formatBytes(int64(summary.PeakSpeed)))
	}
	if summary.Filename != "" {
		fmt.Printf("Saved to: %s\n", summary.Filename)
	}
}

// SetFilename sets the filename for the download summary
func (p *ProgressTracker) SetFilename(filename string) {
	// This will be used in the summary
}

// GetCurrentStats returns current download statistics
func (p *ProgressTracker) GetCurrentStats() (speed float64, eta time.Duration, percentage float64) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	// Calculate current speed from recent samples
	var currentSpeed float64
	if len(p.speedSamples) > 0 {
		// Use average of recent samples
		sampleCount := len(p.speedSamples)
		if sampleCount > 3 {
			sampleCount = 3 // Use last 3 samples for current speed
		}
		for i := len(p.speedSamples) - sampleCount; i < len(p.speedSamples); i++ {
			currentSpeed += p.speedSamples[i]
		}
		currentSpeed /= float64(sampleCount)
	}
	
	// Calculate ETA
	var etaTime time.Duration
	if currentSpeed > 0 && p.total > p.current {
		remainingBytes := p.total - p.current
		etaSeconds := float64(remainingBytes) / currentSpeed
		etaTime = time.Duration(etaSeconds) * time.Second
	}
	
	// Calculate percentage
	var percent float64
	if p.total > 0 {
		percent = float64(p.current) / float64(p.total) * 100
	}
	
	return currentSpeed, etaTime, percent
}

// IsQuiet returns whether the tracker is in quiet mode
func (p *ProgressTracker) IsQuiet() bool {
	return p.quiet
}

// formatBytes formats byte count as human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}