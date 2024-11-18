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

	// Counters for requests
	acceptedCount  int
	deniedCount    int
	non200Count    int
	requestCountMu sync.Mutex
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
	matrix.CholeskyFactorization(matrixip)
	fmt.Println("ACCEPTED")
}

// getClientLimiter retrieves or initializes a rate limiter for a given IP
func getClientLimiter(ip string) server.RateLimiter {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	if client, exists := clients[ip]; exists {
		client.lastSeen = time.Now()
		return client.limiter
	}

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
	case "no_rate_limit":
		limiter = &server.NoRateLimiter{}
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

func main() {
	flag.StringVar(&rateLimitAlgorithm, "algorithm", "token_bucket", "Rate limiting algorithm to use")
	flag.IntVar(&requestsPerSecond, "rate", 10, "Number of requests per second")
	flag.IntVar(&burstLimit, "burst", 5, "Burst limit for the rate limiter")
	flag.DurationVar(&windowSize, "window", time.Second, "Window size for window-based algorithms")
	flag.Parse()

	go cleanupClients()

	go func() {
		for {
			time.Sleep(time.Minute)
			requestCountMu.Lock()
			log.Printf("Total accepted requests: %d, Total denied requests: %d, Non-200 responses: %d", acceptedCount, deniedCount, non200Count)
			requestCountMu.Unlock()
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		limiter := getClientLimiter(ip)

		if !limiter.Allow() {
			requestCountMu.Lock()
			deniedCount++
			non200Count++
			requestCountMu.Unlock()

			w.WriteHeader(http.StatusTooManyRequests)
			log.Printf("[%s] Response sent: Status %d, IP %s", time.Now().Format("2006-01-02 15:04:05"), http.StatusTooManyRequests, ip)
			fmt.Fprint(w, "Rate limit exceeded")
			return
		}

		requestCountMu.Lock()
		acceptedCount++
		requestCountMu.Unlock()

		w.WriteHeader(http.StatusOK)
		log.Printf("[%s] Response sent: Status %d, IP %s", time.Now().Format("2006-01-02 15:04:05"), http.StatusOK, ip)
		fmt.Fprint(w, "Hello from the Go server!")
	})

	fmt.Println("Rate-limiting server running on http://0.0.0.0:80")
	log.Fatal(http.ListenAndServe("0.0.0.0:80", nil))
}

// extractIP extracts the IP address from the request's RemoteAddr
func extractIP(r *http.Request) string {
	ipPort := r.RemoteAddr
	ip := ipPort
	if strings.Contains(ipPort, ":") {
		if strings.Count(ipPort, ":") > 1 {
			ip = strings.Trim(ipPort, "[]")
			colon := strings.LastIndex(ip, ":")
			if colon != -1 {
				ip = ip[:colon]
			}
		} else {
			ip, _, _ = net.SplitHostPort(ipPort)
		}
	}
	return ip
}
