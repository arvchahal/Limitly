package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Config holds the configuration for the client
type Config struct {
	Destination string    `json:"destination"`
	Duration    int       `json:"duration"`  // in seconds
	RateType    string    `json:"rateType"`  // const, linear, sin, exp
	Params      []float64 `json:"params"`    // parameters for rate function
}

// RequestData represents the structure of the data sent in each request
type RequestData struct {
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
}

var (
	totalRequestsSent   int
	totalRequestsServed int
	requestsMu          sync.Mutex
	config              Config
	rateFunc            func(float64) float64 // Dynamic rate function
	stopClient          = make(chan struct{})
	wg                  sync.WaitGroup
)

// trackTermination handles clean shutdown and prints metrics
func trackTermination() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-stop
		close(stopClient) // Signal the client to stop
		wg.Wait()         // Wait for all requests to complete
		printMetrics()    // Print the final metrics
		os.Exit(0)
	}()
}

// printMetrics prints the total requests sent and served
func printMetrics() {
	requestsMu.Lock()
	defer requestsMu.Unlock()
	fmt.Println("\nClient shutting down...")
	fmt.Printf("Total requests sent: %d\n", totalRequestsSent)
	fmt.Printf("Total requests served: %d\n", totalRequestsServed)
}

// sendRequest sends a single POST request to the server
func sendRequest(ctx context.Context) {
	defer wg.Done()

	select {
	case <-ctx.Done():
		// Context canceled, exit the function
		return
	default:
		// Prepare the request data
		data := RequestData{
			Timestamp: time.Now().UnixNano(),
			Message:   "Hello from client!",
		}
		jsonData, err := json.Marshal(data)
		if err != nil {
			fmt.Println("Failed to marshal JSON:", err)
			return
		}

		// Send the POST request
		req, err := http.NewRequestWithContext(ctx, "POST", "http://"+config.Destination, bytes.NewBuffer(jsonData))
		if err != nil {
			fmt.Println("Failed to create request:", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("Failed to send request:", err)
			return
		}
		defer resp.Body.Close()

		// Track the request
		requestsMu.Lock()
		totalRequestsSent++
		if resp.StatusCode == http.StatusOK {
			totalRequestsServed++
		}
		requestsMu.Unlock()
	}
}

// startClient sends requests based on the dynamic rate function
func startClient() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Duration)*time.Second)
	defer cancel()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			// Stop sending requests after the duration
			return
		case <-stopClient:
			// Stop sending requests due to termination
			return
		default:
			// Calculate elapsed time in seconds
			elapsed := time.Since(startTime).Seconds()

			// Get the current rate from the rate function
			currentRate := rateFunc(elapsed)

			// Ensure rate is non-negative
			if currentRate <= 0 {
				time.Sleep(time.Second)
				continue
			}

			// Send requests at the calculated rate
			requestInterval := time.Second / time.Duration(currentRate)
			time.Sleep(requestInterval)

			wg.Add(1)
			go sendRequest(ctx)
		}
	}
}

// initRateFunc initializes the rate function based on rateType and params
func initRateFunc() error {
	switch config.RateType {
	case "const":
		if len(config.Params) < 1 {
			return fmt.Errorf("const rate type requires 1 parameter")
		}
		rateFunc = func(t float64) float64 {
			return config.Params[0]
		}
	case "linear":
		if len(config.Params) < 2 {
			return fmt.Errorf("linear rate type requires 2 parameters")
		}
		m, b := config.Params[0], config.Params[1]
		rateFunc = func(t float64) float64 {
			return m*t + b
		}
	case "sin":
		if len(config.Params) < 3 {
			return fmt.Errorf("sin rate type requires 3 parameters")
		}
		a, b, c := config.Params[0], config.Params[1], config.Params[2]
		rateFunc = func(t float64) float64 {
			return a*math.Sin(t/b) + c
		}
	case "exp":
		if len(config.Params) < 3 {
			return fmt.Errorf("exp rate type requires 3 parameters")
		}
		a, b, c := config.Params[0], config.Params[1], config.Params[2]
		rateFunc = func(t float64) float64 {
			return a*math.Exp(b*t) + c
		}
	default:
		return fmt.Errorf("invalid rate type: %s", config.RateType)
	}
	return nil
}

// loadConfig reads the configuration from the provided JSON file
func loadConfig(configFile string) error {
	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	err = json.Unmarshal(file, &config)
	if err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}
	return nil
}

func main() {
	// Initialize termination tracking
	trackTermination()

	// Parse command-line arguments
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <config_file>")
		os.Exit(1)
	}
	configFile := os.Args[1]

	// Load configuration from the JSON file
	err := loadConfig(configFile)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize rate function
	err = initRateFunc()
	if err != nil {
		fmt.Printf("Error initializing rate function: %v\n", err)
		os.Exit(1)
	}

	// Print configuration
	fmt.Printf("Loaded configuration: Destination=%s, Duration=%d, RateType=%s, Params=%v\n", config.Destination, config.Duration, config.RateType, config.Params)

	// Start sending requests
	fmt.Printf("Sending requests to %s for %d seconds with rate type '%s'...\n", config.Destination, config.Duration, config.RateType)
	startClient()

	// Ensure metrics are printed on normal termination
	printMetrics()
}
