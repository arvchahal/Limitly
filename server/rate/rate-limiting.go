package server

//ratelimiting.go
import (
	"sync"
	"time"
)

// RateLimiter interface defines the Allow method to be used by both algorithms
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
