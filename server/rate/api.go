package server

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

var backendURL string

func SetBackendURL(url string) {
	backendURL = url
}

var rateLimiter RateLimiter

// SetRateLimiter initializes the rate limiter based on parameters
func SetRateLimiter(algorithm string, rate int, burst int) {
	switch algorithm {
	case "token_bucket":
		rateLimiter = NewTokenBucket(burst, time.Second/time.Duration(rate))
	case "leaky_bucket":
		rateLimiter = NewLeakyBucket(burst, time.Second/time.Duration(rate))
	default:
		log.Fatalf("Unknown algorithm: %s", algorithm)
	}
}

// ProxyHandler applies rate limiting and forwards requests
func ProxyHandler(w http.ResponseWriter, r *http.Request) {
	if rateLimiter != nil && !rateLimiter.Allow() {
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return
	}

	targetURL, err := url.Parse(backendURL)
	if err != nil {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ServeHTTP(w, r)
}
