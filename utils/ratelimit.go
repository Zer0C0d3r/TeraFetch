package utils

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"terafetch/internal"
)

// TokenBucketLimiter implements rate limiting using token bucket algorithm
type TokenBucketLimiter struct {
	rate         int64
	bucket       int64
	maxBucket    int64
	lastUpdate   time.Time
	mutex        sync.Mutex
	threadCount  int32 // Number of active threads using this limiter
	threadMutex  sync.RWMutex
	
	// Dynamic rate adjustment
	originalRate    int64
	adjustmentFactor float64
	lastAdjustment   time.Time
	networkStats     *NetworkStats
}

// NetworkStats tracks network performance for dynamic adjustment
type NetworkStats struct {
	mutex           sync.RWMutex
	totalBytes      int64
	totalDuration   time.Duration
	lastMeasurement time.Time
	avgSpeed        float64 // bytes per second
	measurements    []SpeedMeasurement
	maxMeasurements int
}

// SpeedMeasurement represents a single speed measurement
type SpeedMeasurement struct {
	timestamp time.Time
	speed     float64 // bytes per second
}

// NewTokenBucketLimiter creates a new rate limiter
func NewTokenBucketLimiter(bytesPerSecond int64) internal.RateLimiter {
	return &TokenBucketLimiter{
		rate:             bytesPerSecond,
		bucket:           bytesPerSecond,
		maxBucket:        bytesPerSecond,
		lastUpdate:       time.Now(),
		threadCount:      0,
		originalRate:     bytesPerSecond,
		adjustmentFactor: 1.0,
		lastAdjustment:   time.Now(),
		networkStats: &NetworkStats{
			lastMeasurement: time.Now(),
			maxMeasurements: 10, // Keep last 10 measurements for averaging
			measurements:    make([]SpeedMeasurement, 0, 10),
		},
	}
}

// NewDistributedRateLimiter creates a rate limiter that distributes bandwidth across threads
func NewDistributedRateLimiter(bytesPerSecond int64, threadCount int) internal.RateLimiter {
	limiter := NewTokenBucketLimiter(bytesPerSecond).(*TokenBucketLimiter)
	limiter.threadCount = int32(threadCount)
	return limiter
}

// Wait blocks until the specified number of bytes can be consumed
func (r *TokenBucketLimiter) Wait(ctx context.Context, n int) error {
	if r.rate <= 0 {
		return nil // No rate limiting
	}

	startTime := time.Now()
	
	r.mutex.Lock()
	
	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(r.lastUpdate)
	r.lastUpdate = now

	// Calculate effective rate considering thread distribution
	effectiveRate := r.calculateEffectiveRate()

	// Add tokens based on elapsed time and effective rate
	tokensToAdd := int64(elapsed.Seconds() * float64(effectiveRate))
	r.bucket += tokensToAdd
	if r.bucket > r.maxBucket {
		r.bucket = r.maxBucket
	}

	// Check if we have enough tokens
	needed := int64(n)
	if r.bucket >= needed {
		r.bucket -= needed
		r.mutex.Unlock()
		
		// Update network stats for this successful transfer
		transferTime := time.Since(startTime)
		if transferTime > 0 {
			r.UpdateNetworkStats(needed, transferTime)
		}
		return nil
	}

	// Calculate wait time for remaining tokens using effective rate
	deficit := needed - r.bucket
	waitTime := time.Duration(float64(deficit)/float64(effectiveRate)*1000) * time.Millisecond

	// Consume available tokens
	r.bucket = 0
	r.mutex.Unlock()

	// Wait for the required time
	select {
	case <-time.After(waitTime):
		// Update network stats including wait time
		totalTime := time.Since(startTime)
		if totalTime > 0 {
			r.UpdateNetworkStats(needed, totalTime)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// calculateEffectiveRate calculates the effective rate considering thread distribution
func (r *TokenBucketLimiter) calculateEffectiveRate() int64 {
	r.threadMutex.RLock()
	threadCount := r.threadCount
	r.threadMutex.RUnlock()

	if threadCount <= 1 {
		return r.rate
	}

	// Distribute rate across threads, but ensure each thread gets at least some bandwidth
	// Use a fair distribution with a minimum per-thread guarantee
	minPerThread := int64(1024) // 1KB minimum per thread
	totalMinimum := minPerThread * int64(threadCount)

	if r.rate <= totalMinimum {
		// If total rate is less than minimum requirements, give equal share
		return r.rate / int64(threadCount)
	}

	// Otherwise, distribute fairly
	return r.rate
}

// SetRate updates the rate limit
func (r *TokenBucketLimiter) SetRate(bytesPerSecond int64) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	r.rate = bytesPerSecond
	r.originalRate = bytesPerSecond
	r.maxBucket = bytesPerSecond
	if r.bucket > r.maxBucket {
		r.bucket = r.maxBucket
	}
}

// RegisterThread registers a new thread with the rate limiter
func (r *TokenBucketLimiter) RegisterThread() {
	r.threadMutex.Lock()
	defer r.threadMutex.Unlock()
	r.threadCount++
}

// UnregisterThread removes a thread from the rate limiter
func (r *TokenBucketLimiter) UnregisterThread() {
	r.threadMutex.Lock()
	defer r.threadMutex.Unlock()
	if r.threadCount > 0 {
		r.threadCount--
	}
}

// GetThreadCount returns the current number of registered threads
func (r *TokenBucketLimiter) GetThreadCount() int32 {
	r.threadMutex.RLock()
	defer r.threadMutex.RUnlock()
	return r.threadCount
}

// UpdateNetworkStats updates network performance statistics for dynamic adjustment
func (r *TokenBucketLimiter) UpdateNetworkStats(bytesTransferred int64, duration time.Duration) {
	if duration <= 0 || bytesTransferred <= 0 {
		return
	}

	r.networkStats.mutex.Lock()
	defer r.networkStats.mutex.Unlock()

	now := time.Now()
	speed := float64(bytesTransferred) / duration.Seconds()

	// Add new measurement
	measurement := SpeedMeasurement{
		timestamp: now,
		speed:     speed,
	}

	// Keep only recent measurements
	if len(r.networkStats.measurements) >= r.networkStats.maxMeasurements {
		// Remove oldest measurement
		r.networkStats.measurements = r.networkStats.measurements[1:]
	}
	r.networkStats.measurements = append(r.networkStats.measurements, measurement)

	// Update running totals
	r.networkStats.totalBytes += bytesTransferred
	r.networkStats.totalDuration += duration
	r.networkStats.lastMeasurement = now

	// Calculate average speed from recent measurements
	if len(r.networkStats.measurements) > 0 {
		var totalSpeed float64
		for _, m := range r.networkStats.measurements {
			totalSpeed += m.speed
		}
		r.networkStats.avgSpeed = totalSpeed / float64(len(r.networkStats.measurements))
	}

	// Trigger dynamic rate adjustment if needed
	r.adjustRateBasedOnPerformance()
}

// adjustRateBasedOnPerformance dynamically adjusts rate based on network conditions
func (r *TokenBucketLimiter) adjustRateBasedOnPerformance() {
	// Only adjust every 5 seconds to avoid oscillation
	if time.Since(r.lastAdjustment) < 5*time.Second {
		return
	}

	if len(r.networkStats.measurements) < 3 {
		return // Need at least 3 measurements for reliable adjustment
	}

	// Calculate performance metrics
	recentSpeed := r.networkStats.avgSpeed
	targetRate := float64(r.originalRate)

	// If we're consistently achieving less than 80% of target rate, reduce the limit
	// If we're consistently achieving more than 95% of target rate, we can potentially increase
	utilizationRatio := recentSpeed / targetRate

	var newAdjustmentFactor float64
	if utilizationRatio < 0.8 {
		// Network is struggling, reduce rate by 10%
		newAdjustmentFactor = r.adjustmentFactor * 0.9
	} else if utilizationRatio > 0.95 && r.adjustmentFactor < 1.0 {
		// Network is performing well, increase rate by 5%
		newAdjustmentFactor = r.adjustmentFactor * 1.05
		if newAdjustmentFactor > 1.0 {
			newAdjustmentFactor = 1.0 // Don't exceed original rate
		}
	} else {
		return // No adjustment needed
	}

	// Apply the adjustment
	r.adjustmentFactor = newAdjustmentFactor
	newRate := int64(float64(r.originalRate) * r.adjustmentFactor)
	
	// Update rate without changing original rate
	r.rate = newRate
	r.maxBucket = newRate
	if r.bucket > r.maxBucket {
		r.bucket = r.maxBucket
	}
	
	r.lastAdjustment = time.Now()
}

// ParseRateLimit parses human-readable rate limit strings (e.g., "5M", "1G")
func ParseRateLimit(rateStr string) (int64, error) {
	if rateStr == "" {
		return 0, nil
	}

	// Remove whitespace
	rateStr = strings.TrimSpace(rateStr)
	if rateStr == "" {
		return 0, nil
	}

	// Handle pure numbers (bytes per second)
	if val, err := strconv.ParseInt(rateStr, 10, 64); err == nil {
		return val, nil
	}

	// Parse with suffix
	if len(rateStr) < 2 {
		return 0, fmt.Errorf("invalid rate format: %s", rateStr)
	}

	// Extract number and suffix - handle both 1 and 2 character suffixes
	var numStr, suffix string
	rateUpper := strings.ToUpper(rateStr)
	
	// Check for 2-character suffixes first (KB, MB, GB, TB)
	if len(rateUpper) >= 3 && (strings.HasSuffix(rateUpper, "KB") || 
		strings.HasSuffix(rateUpper, "MB") || 
		strings.HasSuffix(rateUpper, "GB") || 
		strings.HasSuffix(rateUpper, "TB")) {
		numStr = rateStr[:len(rateStr)-2]
		suffix = rateUpper[len(rateUpper)-2:]
	} else {
		// Single character suffix (B, K, M, G, T)
		numStr = rateStr[:len(rateStr)-1]
		suffix = rateUpper[len(rateUpper)-1:]
	}

	// Parse the numeric part
	var baseValue float64
	var err error
	if strings.Contains(numStr, ".") {
		baseValue, err = strconv.ParseFloat(numStr, 64)
	} else {
		var intVal int64
		intVal, err = strconv.ParseInt(numStr, 10, 64)
		baseValue = float64(intVal)
	}

	if err != nil {
		return 0, fmt.Errorf("invalid numeric value in rate: %s", numStr)
	}

	if baseValue < 0 {
		return 0, fmt.Errorf("rate cannot be negative: %f", baseValue)
	}

	// Apply multiplier based on suffix
	var multiplier int64
	switch suffix {
	case "B":
		multiplier = 1
	case "K", "KB":
		multiplier = 1024
	case "M", "MB":
		multiplier = 1024 * 1024
	case "G", "GB":
		multiplier = 1024 * 1024 * 1024
	case "T", "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unsupported rate suffix: %s (supported: B, K/KB, M/MB, G/GB, T/TB)", suffix)
	}

	result := int64(baseValue * float64(multiplier))
	if result < 0 {
		return 0, fmt.Errorf("rate value overflow")
	}

	return result, nil
}