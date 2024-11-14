package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"../../rate_limiting"
)

func main() {
	// Command-line arguments
	algorithm := flag.String("algorithm", "token_bucket", "Rate limiting algorithm to use")
	rateLimit := flag.Int("rate", 10, "Rate limit for the algorithm")
	burstLimit := flag.Int("burst", 5, "Burst limit for the algorithm")
	backendURL := flag.String("backend", "http://localhost:8081", "Backend server URL")
	flag.Parse()

	// Set the backend URL
	rate_limiting.SetBackendURL(*backendURL)

	// Initialize the rate limiter
	rate_limiting.SetRateLimiter(*algorithm, *rateLimit, *burstLimit)

	// Start the rate-limiting proxy server
	http.HandleFunc("/", rate_limiting.ProxyHandler)
	fmt.Println("Rate-limiting proxy server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
