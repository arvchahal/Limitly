package server

import (
	"sync"
	"time"
)

// NoRateLimiter struct for no rate limiting
type NoRateLimiter struct{}

// Allow always returns true for NoRateLimiter
func (nrl *NoRateLimiter) Allow() bool {
	return true
}

// RateLimiter interface defines the Allow method to be used by all algorithms
type RateLimiter interface {
	Allow() bool
}

// TokenBucket struct for token bucket algorithm
type TokenBucket struct {
	capacity    int
	tokens      int
	refillRate  time.Duration
	lastRefill  time.Time
	refillMutex sync.Mutex
}

// NewTokenBucket creates a new TokenBucket
func NewTokenBucket(capacity int, refillRate time.Duration) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request can proceed under token bucket algorithm
func (tb *TokenBucket) Allow() bool {
	tb.refillMutex.Lock()
	defer tb.refillMutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	tokensToAdd := int(elapsed / tb.refillRate)
	if tokensToAdd > 0 {
		tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
		tb.lastRefill = now
	}

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}

// LeakyBucket struct for leaky bucket algorithm
type LeakyBucket struct {
	capacity     int
	interval     time.Duration
	lastLeakTime time.Time
	currentCount int
	leakMutex    sync.Mutex
}

// NewLeakyBucket creates a new LeakyBucket
func NewLeakyBucket(capacity int, interval time.Duration) *LeakyBucket {
	return &LeakyBucket{
		capacity:     capacity,
		interval:     interval,
		lastLeakTime: time.Now(),
		currentCount: 0,
	}
}

// Allow checks if a request can proceed under leaky bucket algorithm
func (lb *LeakyBucket) Allow() bool {
	lb.leakMutex.Lock()
	defer lb.leakMutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(lb.lastLeakTime)

	leaks := int(elapsed / lb.interval)
	if leaks > 0 {
		lb.currentCount = max(0, lb.currentCount-leaks)
		lb.lastLeakTime = now
	}

	if lb.currentCount < lb.capacity {
		lb.currentCount++
		return true
	}
	return false
}

// SlidingWindow struct for the sliding window algorithm
type SlidingWindow struct {
	windowSize time.Duration
	limit      int
	timestamps []time.Time
	mutex      sync.Mutex
}

// NewSlidingWindow creates a new SlidingWindow instance
func NewSlidingWindow(limit int, windowSize time.Duration) *SlidingWindow {
	return &SlidingWindow{
		windowSize: windowSize,
		limit:      limit,
		timestamps: make([]time.Time, 0, limit),
	}
}

// Allow checks if a request can proceed under the sliding window algorithm
func (sw *SlidingWindow) Allow() bool {
	sw.mutex.Lock()
	defer sw.mutex.Unlock()

	now := time.Now()
	validWindowStart := now.Add(-sw.windowSize)

	// Prune outdated timestamps
	for len(sw.timestamps) > 0 && sw.timestamps[0].Before(validWindowStart) {
		sw.timestamps = sw.timestamps[1:]
	}

	// Check if within limit
	if len(sw.timestamps) < sw.limit {
		sw.timestamps = append(sw.timestamps, now)
		return true
	}

	return false
}

// FixedWindow struct for the fixed window algorithm
type FixedWindow struct {
	windowSize  time.Duration
	limit       int
	count       int
	windowStart time.Time
	mutex       sync.Mutex
}

// NewFixedWindow creates a new FixedWindow instance
func NewFixedWindow(limit int, windowSize time.Duration) *FixedWindow {
	return &FixedWindow{
		windowSize:  windowSize,
		limit:       limit,
		count:       0,
		windowStart: time.Now(),
	}
}

// Allow checks if a request can proceed under the fixed window algorithm
func (fw *FixedWindow) Allow() bool {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	now := time.Now()

	// Check if we are still in the current window
	if now.Sub(fw.windowStart) >= fw.windowSize {
		// Reset the window
		fw.windowStart = now
		fw.count = 0
	}

	// Check if within limit
	if fw.count < fw.limit {
		fw.count++
		return true
	}

	return false
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
