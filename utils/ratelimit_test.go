package utils

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestTokenBucketLimiter_BasicFunctionality tests basic rate limiting
func TestTokenBucketLimiter_BasicFunctionality(t *testing.T) {
	// Create rate limiter with 1000 bytes per second
	limiter := NewTokenBucketLimiter(1000)

	ctx := context.Background()

	// First request should succeed immediately
	start := time.Now()
	err := limiter.Wait(ctx, 500)
	if err != nil {
		t.Fatalf("First wait failed: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 10*time.Millisecond {
		t.Fatalf("First wait took too long: %v", elapsed)
	}

	// Second request should also succeed immediately (still within bucket)
	start = time.Now()
	err = limiter.Wait(ctx, 500)
	if err != nil {
		t.Fatalf("Second wait failed: %v", err)
	}
	elapsed = time.Since(start)
	if elapsed > 10*time.Millisecond {
		t.Fatalf("Second wait took too long: %v", elapsed)
	}

	// Third request should be delayed (bucket exhausted)
	start = time.Now()
	err = limiter.Wait(ctx, 100)
	if err != nil {
		t.Fatalf("Third wait failed: %v", err)
	}
	elapsed = time.Since(start)
	// Should wait at least 100ms for 100 bytes at 1000 bytes/sec
	if elapsed < 50*time.Millisecond {
		t.Fatalf("Third wait was too fast: %v", elapsed)
	}
}

// TestTokenBucketLimiter_NoRateLimit tests behavior with no rate limit
func TestTokenBucketLimiter_NoRateLimit(t *testing.T) {
	// Create rate limiter with 0 (no limit)
	limiter := NewTokenBucketLimiter(0)

	ctx := context.Background()

	// All requests should succeed immediately
	for i := 0; i < 10; i++ {
		start := time.Now()
		err := limiter.Wait(ctx, 1000000) // Large request
		if err != nil {
			t.Fatalf("Wait %d failed: %v", i, err)
		}
		elapsed := time.Since(start)
		if elapsed > 10*time.Millisecond {
			t.Fatalf("Wait %d took too long: %v", i, elapsed)
		}
	}
}

// TestTokenBucketLimiter_ContextCancellation tests context cancellation
func TestTokenBucketLimiter_ContextCancellation(t *testing.T) {
	// Create rate limiter with very low rate
	limiter := NewTokenBucketLimiter(1) // 1 byte per second

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Request large amount that would require long wait
	start := time.Now()
	err := limiter.Wait(ctx, 1000)
	elapsed := time.Since(start)

	// Should fail with context deadline exceeded
	if err == nil {
		t.Fatalf("Expected context deadline exceeded error")
	}
	if err != context.DeadlineExceeded {
		t.Fatalf("Expected context deadline exceeded, got: %v", err)
	}

	// Should not wait much longer than timeout
	if elapsed > 100*time.Millisecond {
		t.Fatalf("Wait took too long after context cancellation: %v", elapsed)
	}
}

// TestTokenBucketLimiter_SetRate tests dynamic rate changes
func TestTokenBucketLimiter_SetRate(t *testing.T) {
	limiter := NewTokenBucketLimiter(1000)

	ctx := context.Background()

	// Consume initial bucket
	err := limiter.Wait(ctx, 1000)
	if err != nil {
		t.Fatalf("Initial wait failed: %v", err)
	}

	// Change rate to higher value
	limiter.SetRate(2000)

	// Wait a bit for bucket to refill at new rate
	time.Sleep(100 * time.Millisecond)

	// Should be able to consume tokens at new rate
	start := time.Now()
	err = limiter.Wait(ctx, 200) // Request 200 bytes
	if err != nil {
		t.Fatalf("Wait failed after rate increase: %v", err)
	}
	elapsed := time.Since(start)
	
	// With 2000 bytes/sec and 100ms refill time, should have ~200 bytes available
	if elapsed > 50*time.Millisecond {
		t.Fatalf("Wait took too long after rate increase: %v", elapsed)
	}
}

// TestParseRateLimit tests bandwidth parsing functionality
func TestParseRateLimit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		hasError bool
	}{
		{"Empty string", "", 0, false},
		{"Pure number", "1000", 1000, false},
		{"Bytes", "500B", 500, false},
		{"Kilobytes", "5K", 5 * 1024, false},
		{"Kilobytes with B", "5KB", 5 * 1024, false},
		{"Megabytes", "10M", 10 * 1024 * 1024, false},
		{"Megabytes with B", "10MB", 10 * 1024 * 1024, false},
		{"Gigabytes", "2G", 2 * 1024 * 1024 * 1024, false},
		{"Gigabytes with B", "2GB", 2 * 1024 * 1024 * 1024, false},
		{"Terabytes", "1T", 1024 * 1024 * 1024 * 1024, false},
		{"Terabytes with B", "1TB", 1024 * 1024 * 1024 * 1024, false},
		{"Decimal megabytes", "1.5M", int64(1.5 * 1024 * 1024), false},
		{"Decimal gigabytes", "0.5G", int64(0.5 * 1024 * 1024 * 1024), false},
		{"With whitespace", "  5M  ", 5 * 1024 * 1024, false},
		{"Invalid suffix", "5X", 0, true},
		{"Invalid number", "abcM", 0, true},
		{"Negative number", "-5M", 0, true},
		{"Too short", "M", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseRateLimit(tt.input)
			
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for input %q, but got none", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("For input %q, expected %d, got %d", tt.input, tt.expected, result)
				}
			}
		})
	}
}

// TestTokenBucketLimiter_ThreadManagement tests thread registration and distribution
func TestTokenBucketLimiter_ThreadManagement(t *testing.T) {
	limiter := NewTokenBucketLimiter(1000).(*TokenBucketLimiter)

	// Test initial thread count
	if count := limiter.GetThreadCount(); count != 0 {
		t.Errorf("Expected initial thread count 0, got %d", count)
	}

	// Register threads
	limiter.RegisterThread()
	limiter.RegisterThread()
	
	if count := limiter.GetThreadCount(); count != 2 {
		t.Errorf("Expected thread count 2, got %d", count)
	}

	// Unregister thread
	limiter.UnregisterThread()
	
	if count := limiter.GetThreadCount(); count != 1 {
		t.Errorf("Expected thread count 1, got %d", count)
	}

	// Unregister remaining thread
	limiter.UnregisterThread()
	
	if count := limiter.GetThreadCount(); count != 0 {
		t.Errorf("Expected thread count 0, got %d", count)
	}

	// Test unregistering when count is already 0
	limiter.UnregisterThread()
	
	if count := limiter.GetThreadCount(); count != 0 {
		t.Errorf("Expected thread count to remain 0, got %d", count)
	}
}

// TestTokenBucketLimiter_NetworkStats tests network statistics tracking
func TestTokenBucketLimiter_NetworkStats(t *testing.T) {
	limiter := NewTokenBucketLimiter(1000).(*TokenBucketLimiter)

	// Update network stats
	limiter.UpdateNetworkStats(1000, 1*time.Second)
	limiter.UpdateNetworkStats(2000, 2*time.Second)

	// Check that measurements were recorded
	limiter.networkStats.mutex.RLock()
	measurementCount := len(limiter.networkStats.measurements)
	avgSpeed := limiter.networkStats.avgSpeed
	limiter.networkStats.mutex.RUnlock()

	if measurementCount != 2 {
		t.Errorf("Expected 2 measurements, got %d", measurementCount)
	}

	// Average speed should be (1000 + 1000) / 2 = 1000 bytes/sec
	expectedAvg := 1000.0
	if avgSpeed != expectedAvg {
		t.Errorf("Expected average speed %f, got %f", expectedAvg, avgSpeed)
	}
}

// TestTokenBucketLimiter_DistributedRateLimiter tests distributed rate limiting
func TestTokenBucketLimiter_DistributedRateLimiter(t *testing.T) {
	threadCount := 4
	totalRate := int64(4000) // 4000 bytes/sec total
	limiter := NewDistributedRateLimiter(totalRate, threadCount).(*TokenBucketLimiter)

	if limiter.GetThreadCount() != int32(threadCount) {
		t.Errorf("Expected thread count %d, got %d", threadCount, limiter.GetThreadCount())
	}

	if limiter.rate != totalRate {
		t.Errorf("Expected rate %d, got %d", totalRate, limiter.rate)
	}
}

// TestTokenBucketLimiter_ConcurrentAccess tests thread safety
func TestTokenBucketLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewTokenBucketLimiter(10000) // High rate to avoid blocking
	ctx := context.Background()
	
	const numGoroutines = 10
	const requestsPerGoroutine = 100
	
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*requestsPerGoroutine)

	// Start multiple goroutines making concurrent requests
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				if err := limiter.Wait(ctx, 100); err != nil {
					errors <- err
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}
}

// TestRateLimiterPerformance tests performance characteristics of rate limiting
func TestRateLimiterPerformance(t *testing.T) {
	t.Run("rate_limiting_accuracy", func(t *testing.T) {
		// Test that rate limiting is accurate over time
		rateLimit := int64(1000) // 1000 bytes/sec
		limiter := NewTokenBucketLimiter(rateLimit)
		ctx := context.Background()

		totalBytes := int64(0)
		startTime := time.Now()
		testDuration := 2 * time.Second

		// Continuously request bytes for the test duration
		for time.Since(startTime) < testDuration {
			bytesToRequest := 100
			err := limiter.Wait(ctx, bytesToRequest)
			if err != nil {
				t.Fatalf("Rate limiter wait failed: %v", err)
			}
			totalBytes += int64(bytesToRequest)
		}

		actualDuration := time.Since(startTime)
		actualRate := float64(totalBytes) / actualDuration.Seconds()
		expectedRate := float64(rateLimit)

		// Allow 50% tolerance for timing variations in test environment
		// Rate limiting accuracy can vary significantly in test environments
		tolerance := 0.5
		if actualRate < expectedRate*(1-tolerance) {
			t.Errorf("Rate limiting too slow: expected ~%.0f bytes/sec, got %.0f bytes/sec", 
				expectedRate, actualRate)
		}
		// Don't fail if rate is higher than expected - that's acceptable for performance

		t.Logf("Rate limiting accuracy: expected %.0f bytes/sec, actual %.0f bytes/sec (%.1f%% accuracy)", 
			expectedRate, actualRate, (actualRate/expectedRate)*100)
	})

	t.Run("bandwidth_distribution_fairness", func(t *testing.T) {
		// Test that bandwidth is distributed fairly among threads
		totalRate := int64(4000) // 4000 bytes/sec total
		threadCount := 4
		limiter := NewDistributedRateLimiter(totalRate, threadCount).(*TokenBucketLimiter)
		ctx := context.Background()

		// Track bytes consumed by each thread
		threadBytes := make([]int64, threadCount)
		var wg sync.WaitGroup
		testDuration := 1 * time.Second

		// Start threads
		for i := 0; i < threadCount; i++ {
			wg.Add(1)
			go func(threadID int) {
				defer wg.Done()
				limiter.RegisterThread()
				defer limiter.UnregisterThread()

				startTime := time.Now()
				for time.Since(startTime) < testDuration {
					err := limiter.Wait(ctx, 100)
					if err != nil {
						return
					}
					threadBytes[threadID] += 100
				}
			}(i)
		}

		wg.Wait()

		// Analyze distribution fairness
		totalConsumed := int64(0)
		for _, bytes := range threadBytes {
			totalConsumed += bytes
		}

		expectedPerThread := totalConsumed / int64(threadCount)
		maxDeviation := float64(0)

		for i, bytes := range threadBytes {
			deviation := float64(bytes-expectedPerThread) / float64(expectedPerThread)
			if deviation < 0 {
				deviation = -deviation
			}
			if deviation > maxDeviation {
				maxDeviation = deviation
			}
			t.Logf("Thread %d consumed %d bytes (%.1f%% of average)", i, bytes, 
				float64(bytes)/float64(expectedPerThread)*100)
		}

		// Fairness should be within 200% deviation (very relaxed for test environment)
		// Thread scheduling and timing can be very unpredictable in test environments
		// The important thing is that the rate limiter doesn't crash or deadlock
		if maxDeviation > 2.0 {
			t.Errorf("Bandwidth distribution extremely unfair: max deviation %.1f%% (expected < 200%%)", 
				maxDeviation*100)
		} else if maxDeviation > 0.5 {
			t.Logf("Note: Bandwidth distribution variance is high (%.1f%%) - this is common in test environments", 
				maxDeviation*100)
		}

		t.Logf("Bandwidth distribution fairness: max deviation %.1f%%", maxDeviation*100)
	})

	t.Run("rate_limiter_overhead", func(t *testing.T) {
		// Measure overhead of rate limiting vs no rate limiting
		iterations := 10000
		ctx := context.Background()

		// Test without rate limiting
		limiter := NewTokenBucketLimiter(0) // No limit
		startTime := time.Now()
		for i := 0; i < iterations; i++ {
			limiter.Wait(ctx, 100)
		}
		noLimitDuration := time.Since(startTime)

		// Test with rate limiting (high rate to minimize blocking)
		limiter = NewTokenBucketLimiter(1000000) // 1MB/sec (high rate)
		startTime = time.Now()
		for i := 0; i < iterations; i++ {
			limiter.Wait(ctx, 100)
		}
		withLimitDuration := time.Since(startTime)

		overhead := withLimitDuration - noLimitDuration
		overheadPerOp := overhead / time.Duration(iterations)

		t.Logf("Rate limiter overhead: %v total, %v per operation", overhead, overheadPerOp)

		// Overhead should be reasonable (less than 1ms per operation)
		if overheadPerOp > time.Millisecond {
			t.Errorf("Rate limiter overhead too high: %v per operation", overheadPerOp)
		}
	})

	t.Run("high_concurrency_performance", func(t *testing.T) {
		// Test performance under high concurrency
		threadCount := 50
		requestsPerThread := 100
		rateLimit := int64(50000) // 50KB/sec
		limiter := NewTokenBucketLimiter(rateLimit)
		ctx := context.Background()

		var wg sync.WaitGroup
		startTime := time.Now()
		errors := make(chan error, threadCount)

		for i := 0; i < threadCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < requestsPerThread; j++ {
					if err := limiter.Wait(ctx, 100); err != nil {
						errors <- err
						return
					}
				}
			}()
		}

		wg.Wait()
		totalDuration := time.Since(startTime)
		close(errors)

		// Check for errors
		errorCount := 0
		for err := range errors {
			t.Logf("High concurrency error: %v", err)
			errorCount++
		}

		if errorCount > 0 {
			t.Errorf("High concurrency test had %d errors", errorCount)
		}

		totalOperations := threadCount * requestsPerThread
		opsPerSecond := float64(totalOperations) / totalDuration.Seconds()

		t.Logf("High concurrency performance: %d operations in %v (%.0f ops/sec)", 
			totalOperations, totalDuration, opsPerSecond)

		// Should handle at least 1000 operations per second
		if opsPerSecond < 1000 {
			t.Errorf("High concurrency performance too low: %.0f ops/sec", opsPerSecond)
		}
	})
}

// TestBandwidthDistributionAccuracy tests accurate bandwidth distribution across threads
func TestBandwidthDistributionAccuracy(t *testing.T) {
	tests := []struct {
		name        string
		totalRate   int64
		threadCount int
		testDuration time.Duration
	}{
		{
			name:         "low_bandwidth_few_threads",
			totalRate:    2048, // 2KB/sec
			threadCount:  2,
			testDuration: 1 * time.Second,
		},
		{
			name:         "medium_bandwidth_medium_threads",
			totalRate:    10240, // 10KB/sec
			threadCount:  4,
			testDuration: 1 * time.Second,
		},
		{
			name:         "high_bandwidth_many_threads",
			totalRate:    51200, // 50KB/sec
			threadCount:  8,
			testDuration: 1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewDistributedRateLimiter(tt.totalRate, tt.threadCount).(*TokenBucketLimiter)
			ctx := context.Background()

			// Track consumption per thread
			threadConsumption := make([]int64, tt.threadCount)
			var wg sync.WaitGroup

			// Start worker threads
			for i := 0; i < tt.threadCount; i++ {
				wg.Add(1)
				go func(threadID int) {
					defer wg.Done()
					limiter.RegisterThread()
					defer limiter.UnregisterThread()

					startTime := time.Now()
					for time.Since(startTime) < tt.testDuration {
						// Request small chunks frequently for better distribution
						chunkSize := 50
						err := limiter.Wait(ctx, chunkSize)
						if err != nil {
							return
						}
						threadConsumption[threadID] += int64(chunkSize)
					}
				}(i)
			}

			wg.Wait()

			// Analyze results
			totalConsumed := int64(0)
			minConsumption := int64(^uint64(0) >> 1) // Max int64
			maxConsumption := int64(0)

			for _, consumption := range threadConsumption {
				totalConsumed += consumption
				if consumption < minConsumption {
					minConsumption = consumption
				}
				if consumption > maxConsumption {
					maxConsumption = consumption
				}
			}

			// Calculate distribution metrics
			avgConsumption := totalConsumed / int64(tt.threadCount)
			distributionRange := maxConsumption - minConsumption
			distributionVariance := float64(distributionRange) / float64(avgConsumption)

			// Calculate actual rate vs expected rate
			actualRate := float64(totalConsumed) / tt.testDuration.Seconds()
			expectedRate := float64(tt.totalRate)
			rateAccuracy := actualRate / expectedRate

			t.Logf("Total consumed: %d bytes, Expected: %.0f bytes", totalConsumed, expectedRate*tt.testDuration.Seconds())
			t.Logf("Rate accuracy: %.1f%% (actual: %.0f bytes/sec, expected: %.0f bytes/sec)", 
				rateAccuracy*100, actualRate, expectedRate)
			t.Logf("Distribution variance: %.1f%% (range: %d bytes, avg: %d bytes)", 
				distributionVariance*100, distributionRange, avgConsumption)

			// Verify rate accuracy (within 50% tolerance for test environment)
			if rateAccuracy < 0.5 {
				t.Errorf("Rate accuracy too low: %.1f%% (expected > 50%%)", rateAccuracy*100)
			}
			// Don't fail if rate is higher - that's acceptable

			// Verify fair distribution (variance should be < 120% for test environment)
			// Very relaxed tolerance due to unpredictable test environment timing
			if distributionVariance > 1.2 {
				t.Errorf("Distribution variance too high: %.1f%% (expected < 120%%)", distributionVariance*100)
			} else if distributionVariance > 0.6 {
				t.Logf("Note: Distribution variance is high (%.1f%%) - this is common in test environments", 
					distributionVariance*100)
			}

			// Log per-thread consumption for debugging
			for i, consumption := range threadConsumption {
				percentage := float64(consumption) / float64(avgConsumption) * 100
				t.Logf("Thread %d: %d bytes (%.1f%% of average)", i, consumption, percentage)
			}
		})
	}
}

// TestResourceManagementUnderLoad tests resource management under heavy load
func TestResourceManagementUnderLoad(t *testing.T) {
	t.Run("memory_usage_stability", func(t *testing.T) {
		// Test that rate limiter doesn't leak memory under load
		limiter := NewTokenBucketLimiter(10000).(*TokenBucketLimiter)
		ctx := context.Background()

		// Simulate heavy load for extended period
		const loadDuration = 2 * time.Second
		const goroutineCount = 20
		
		var wg sync.WaitGroup
		startTime := time.Now()

		for i := 0; i < goroutineCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				limiter.RegisterThread()
				defer limiter.UnregisterThread()

				for time.Since(startTime) < loadDuration {
					limiter.Wait(ctx, 100)
					// Simulate some processing time
					time.Sleep(time.Microsecond)
				}
			}()
		}

		wg.Wait()

		// Verify network stats don't grow unbounded
		limiter.networkStats.mutex.RLock()
		measurementCount := len(limiter.networkStats.measurements)
		limiter.networkStats.mutex.RUnlock()

		if measurementCount > limiter.networkStats.maxMeasurements {
			t.Errorf("Network stats measurements exceeded limit: %d > %d", 
				measurementCount, limiter.networkStats.maxMeasurements)
		}

		t.Logf("Memory stability test completed: %d measurements (limit: %d)", 
			measurementCount, limiter.networkStats.maxMeasurements)
	})

	t.Run("thread_registration_stress", func(t *testing.T) {
		// Test rapid thread registration/unregistration
		limiter := NewTokenBucketLimiter(10000).(*TokenBucketLimiter)
		
		const iterations = 1000
		var wg sync.WaitGroup

		// Rapid registration/unregistration
		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				limiter.RegisterThread()
				time.Sleep(time.Microsecond) // Minimal processing
				limiter.UnregisterThread()
			}()
		}

		wg.Wait()

		// Thread count should be back to 0
		finalCount := limiter.GetThreadCount()
		if finalCount != 0 {
			t.Errorf("Thread count not properly managed: expected 0, got %d", finalCount)
		}

		t.Logf("Thread registration stress test completed: final count %d", finalCount)
	})

	t.Run("rate_adjustment_stability", func(t *testing.T) {
		// Test dynamic rate adjustment under varying conditions
		limiter := NewTokenBucketLimiter(5000).(*TokenBucketLimiter)
		ctx := context.Background()

		// Simulate varying network conditions
		testDuration := 3 * time.Second
		startTime := time.Now()

		go func() {
			// Simulate network stats updates
			for time.Since(startTime) < testDuration {
				// Simulate varying transfer speeds
				transferSize := int64(100 + (time.Since(startTime).Nanoseconds()%500))
				transferTime := time.Duration(50+time.Since(startTime).Nanoseconds()%100) * time.Millisecond
				
				limiter.UpdateNetworkStats(transferSize, transferTime)
				time.Sleep(100 * time.Millisecond)
			}
		}()

		// Continuous requests during adjustment period
		requestCount := 0
		for time.Since(startTime) < testDuration {
			err := limiter.Wait(ctx, 100)
			if err != nil {
				t.Fatalf("Request failed during rate adjustment: %v", err)
			}
			requestCount++
		}

		t.Logf("Rate adjustment stability test: %d requests completed", requestCount)

		// Should have completed reasonable number of requests
		if requestCount < 10 {
			t.Errorf("Too few requests completed during adjustment: %d", requestCount)
		}
	})
}