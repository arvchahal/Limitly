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

	"golang.org/x/time/rate"
)

// Client represents a client with a rate limiter
type Client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	clients   = make(map[string]*Client)
	clientsMu sync.Mutex

	// Rate limit parameters (modifiable via flags)
	requestsPerSecond = 10
	burstLimit        = 5
)

// Example function to process the request
func customFunction(r *http.Request) {
	fmt.Println("Custom function executed")
	// Add your custom logic here
	// For example, log request details
	fmt.Printf("Request received: Method=%s, URL=%s\n", r.Method, r.URL)
}

// getClientLimiter retrieves or initializes a rate limiter for a given IP
func getClientLimiter(ip string) *rate.Limiter {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	// Check if the client already has a limiter
	if client, exists := clients[ip]; exists {
		client.lastSeen = time.Now()
		return client.limiter
	}

	// If not, create a new limiter and add to the map
	limiter := rate.NewLimiter(rate.Limit(requestsPerSecond), burstLimit)
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
	// algorithm := flag.String("algorithm", "token_bucket", "Rate limiting algorithm to use") // Placeholder
	rateLimit := flag.Int("rate", 10, "Number of requests per second")
	burstLimitFlag := flag.Int("burst", 5, "Burst limit for the rate limiter")
	flag.Parse()

	// Update rate limit parameters if provided
	if *rateLimit > 0 {
		requestsPerSecond = *rateLimit
	}
	if *burstLimitFlag > 0 {
		burstLimit = *burstLimitFlag
	}

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
			return
		}

		// Call your custom function
		customFunction(r)

		// Handle the request directly
		fmt.Fprintf(w, "Hello from the Go server!")
	})

	fmt.Println("Rate-limiting server running on http://0.0.0.0:8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
