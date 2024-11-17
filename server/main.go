package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	matrix "github.com/arvchahal/Limitly/server/matrix"

	server "github.com/arvchahal/Limitly/server/rate" // Import your custom rate-limiting package
)

// Client represents a client with a rate limiter
type Client struct {
	limiter  server.RateLimiter
	lastSeen time.Time
}

var (
	clients   = make(map[string]*Client)
	clientsMu sync.Mutex

	// Rate limit parameters (modifiable via flags)
	rateLimitAlgorithm = "token_bucket" // Default algorithm
	requestsPerSecond  = 10
	burstLimit         = 5
	windowSize         = time.Second
)

// Example function to process the request
func customFunction(r *http.Request) {
	matrixip := [][]float64{
		{10.0, 2.0, 3.0, 4.0, 1.0, 0.5, 0.2, 0.1, 0.1, 0.3},
		{2.0, 9.0, 1.5, 3.0, 1.0, 0.4, 0.2, 0.2, 0.1, 0.5},
		{3.0, 1.5, 8.0, 2.0, 1.0, 0.3, 0.4, 0.1, 0.2, 0.3},
		{4.0, 3.0, 2.0, 12.0, 2.0, 1.0, 0.5, 0.3, 0.2, 0.4},
		{1.0, 1.0, 1.0, 2.0, 7.0, 0.5, 0.4, 0.3, 0.2, 0.5},
		{0.5, 0.4, 0.3, 1.0, 0.5, 6.0, 1.0, 0.4, 0.3, 0.2},
		{0.2, 0.2, 0.4, 0.5, 0.4, 1.0, 5.0, 0.3, 0.2, 0.1},
		{0.1, 0.2, 0.1, 0.3, 0.3, 0.4, 0.3, 4.0, 0.2, 0.2},
		{0.1, 0.1, 0.2, 0.2, 0.2, 0.3, 0.2, 0.2, 3.0, 0.1},
		{0.3, 0.5, 0.3, 0.4, 0.5, 0.2, 0.1, 0.2, 0.1, 2.5},
	}
	z, _ := matrix.CholeskyFactorization(matrixip)
	fmt.Println(z)
	// Add your custom logic here
	// For example, log request details
	// fmt.Printf("Request received: Method=%s, URL=%s\n", r.Method, r.URL)
}

// getClientLimiter retrieves or initializes a rate limiter for a given IP
func getClientLimiter(ip string) server.RateLimiter {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	// Check if the client already has a limiter
	if client, exists := clients[ip]; exists {
		client.lastSeen = time.Now()
		return client.limiter
	}

	// Initialize the appropriate rate limiter based on the selected algorithm
	var limiter server.RateLimiter
	switch rateLimitAlgorithm {
	case "token_bucket":
		limiter = server.NewTokenBucket(burstLimit, time.Second/time.Duration(requestsPerSecond))
	case "leaky_bucket":
		limiter = server.NewLeakyBucket(burstLimit, time.Second/time.Duration(requestsPerSecond))
	case "sliding_window":
		limiter = server.NewSlidingWindow(requestsPerSecond, windowSize)
	case "fixed_window":
		limiter = server.NewFixedWindow(requestsPerSecond, windowSize)
	default:
		log.Fatalf("Unknown rate limiting algorithm: %s", rateLimitAlgorithm)
	}

	clients[ip] = &Client{
		limiter:  limiter,
		lastSeen: time.Now(),
	}
	return limiter
}

// cleanupClients periodically removes clients that haven't been seen for a while
func cleanupClients() {
	for {
		time.Sleep(time.Minute)
		clientsMu.Lock()
		for ip, client := range clients {
			if time.Since(client.lastSeen) > 5*time.Minute {
				delete(clients, ip)
			}
		}
		clientsMu.Unlock()
	}
}

// extractIP extracts the IP address from the request's RemoteAddr
func extractIP(r *http.Request) string {
	ipPort := r.RemoteAddr
	ip := ipPort
	// In some cases, RemoteAddr might not contain a port
	if strings.Contains(ipPort, ":") {
		// Handle IPv6 addresses
		if strings.Count(ipPort, ":") > 1 {
			// IPv6: [::1]:8080
			ip = strings.Trim(ipPort, "[]")
			colon := strings.LastIndex(ip, ":")
			if colon != -1 {
				ip = ip[:colon]
			}
		} else {
			// IPv4: 192.168.1.1:8080
			ip, _, _ = net.SplitHostPort(ipPort)
		}
	}
	return ip
}

func main() {
	// Command-line arguments
	flag.StringVar(&rateLimitAlgorithm, "algorithm", "token_bucket", "Rate limiting algorithm to use (token_bucket, leaky_bucket, sliding_window, fixed_window)")
	flag.IntVar(&requestsPerSecond, "rate", 10, "Number of requests per second")
	flag.IntVar(&burstLimit, "burst", 5, "Burst limit for the rate limiter")
	flag.DurationVar(&windowSize, "window", time.Second, "Window size for window-based algorithms")
	flag.Parse()

	// Start the cleanup goroutine
	go cleanupClients()

	// Start the rate-limiting server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Extract client IP address
		ip := extractIP(r)

		// Get the rate limiter for this IP
		limiter := getClientLimiter(ip)

		// Check if the request is allowed
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			fmt.Println("dead")
			return
		}

		// Call your custom function
		customFunction(r)

		// Handle the request directly
		fmt.Fprintf(w, "Hello from the Go server!")
	})

	fmt.Println("Rate-limiting server running on http://0.0.0.0:80")
	log.Fatal(http.ListenAndServe("0.0.0.0:80", nil))
}
