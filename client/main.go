package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"
)

// Config represents the structure of the configuration file.
// It now includes a RateFunc field to hold the rate-limiting function.
type Config struct {
	Destination string    `json:"destination"`
	Duration    float64   `json:"duration"`
	RateType    string    `json:"rateType"`
	Params      []float64 `json:"params"`
	RateFunc    func(float64) float64
}

// IntervalData holds the elapsed time and the number of requests sent in that interval.
type IntervalData struct {
	ElapsedTimeMs float64
	NumRequests   int
}

// loadConfig reads a JSON configuration file and unmarshals it into a Config struct.
// It then initializes the RateFunc based on the RateType and Params.
func loadConfig(configFile string) (Config, error) {
	var config Config

	// Read the configuration file
	file, err := os.ReadFile(configFile)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal JSON into Config struct
	err = json.Unmarshal(file, &config)
	if err != nil {
		return config, fmt.Errorf("failed to unmarshal config JSON: %w", err)
	}

	// Initialize the RateFunc based on RateType and Params
	rateFunc, err := config.initRateFunc()
	if err != nil {
		return config, err
	}
	config.RateFunc = rateFunc

	return config, nil
}

// initRateFunc initializes the RateFunc field based on the RateType and Params.
// It returns an error if the RateType is invalid or if required parameters are missing.
func (c *Config) initRateFunc() (func(float64) float64, error) {
	rateType := c.RateType
	params := c.Params

	var rateFunc func(float64) float64

	// Determine the required number of parameters based on the rate type
	var requiredParams int
	switch rateType {
	case "const":
		requiredParams = 1
	case "linear":
		requiredParams = 2
	case "sin":
		requiredParams = 3
	case "exp":
		requiredParams = 3
	default:
		return nil, errors.New("invalid rate type; must be const, linear, sin, or exp")
	}

	// Check if the provided parameters meet the required number
	if len(params) < requiredParams {
		return nil, fmt.Errorf("%s rate type requires %d parameter(s)", rateType, requiredParams)
	}

	// Define the rate function based on the rate type
	switch rateType {
	case "const":
		a := params[0]
		rateFunc = func(t float64) float64 {
			return a
		}
	case "linear":
		m := params[0]
		b := params[1]
		rateFunc = func(t float64) float64 {
			return m*t + b
		}
	case "sin":
		a := params[0]
		b := params[1]
		c := params[2]
		rateFunc = func(t float64) float64 {
			return a*math.Sin(t/b) + c
		}
	case "exp":
		a := params[0]
		b := params[1]
		c := params[2]
		rateFunc = func(t float64) float64 {
			return a*math.Exp(b*t) + c
		}
	}

	return rateFunc, nil
}

// sendRequests sends HTTP POST requests with garbage data to the destination.
// It operates for the duration specified in the Config, sending requests at a rate determined by RateFunc.
// Additionally, it logs the elapsed time and number of requests for each interval to an output file.
func sendRequests(config Config) error {
	// Define the total duration and the interval for rate calculations
	totalDuration := time.Duration(config.Duration * float64(time.Second))
	interval := 100 * time.Millisecond // 100ms intervals

	// Create a context that will be canceled after the total duration
	ctx, cancel := context.WithTimeout(context.Background(), totalDuration)
	defer cancel()

	// Define the number of workers
	numWorkers := 100
	var wg sync.WaitGroup

	// Create a buffered channel to queue request jobs
	requestChan := make(chan struct{}, 1000) // Buffered to prevent blocking

	// Create an HTTP client with a timeout
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Define a slice to hold interval data
	var intervals []IntervalData
	var intervalsMutex sync.Mutex // To protect concurrent access to intervals slice

	// Initialize the total number of requests sent
	totalRequests := 0

	// Worker function without id
	worker := func() {
		defer wg.Done()
		for range requestChan {
			// Generate some garbage data (e.g., random JSON)
			data := map[string]interface{}{
				"timestamp": time.Now().UnixNano(),
				"value":     rand.Float64(),
			}
			jsonData, err := json.Marshal(data)
			if err != nil {
				// If marshaling fails, skip this request
				continue
			}

			// Create a new HTTP POST request
			req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("http://%s", config.Destination), bytes.NewBuffer(jsonData))
			if err != nil {
				// If request creation fails, skip this request
				continue
			}
			req.Header.Set("Content-Type", "application/json")

			// Send the HTTP request
			resp, err := httpClient.Do(req)
			if err != nil {
				// Handle the error as needed (e.g., log it)
				continue
			}

			// It's important to close the response body to prevent resource leaks
			resp.Body.Close()
		}
	}

	// Launch workers
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go worker()
	}

	// Start the ticker for intervals
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Record the start time
	startTime := time.Now()

	// Variable to accumulate fractional requests
	var fractionalRequests float64

	// Main loop to send requests based on the rate function
loop:
	for {
		select {
		case <-ctx.Done():
			// Time is up; stop sending requests
			close(requestChan)
			wg.Wait()
			break loop // Exit the loop to proceed to CSV writing
		case tickTime := <-ticker.C:
			// Calculate elapsed time in seconds
			elapsed := tickTime.Sub(startTime).Seconds()
			if elapsed > config.Duration {
				// Exceeded duration; stop sending
				close(requestChan)
				wg.Wait()
				break loop // Exit the loop to proceed to CSV writing
			}

			// Get the current rate from the rate function
			currentRate := config.RateFunc(elapsed)

			// Ensure that the rate is non-negative
			if currentRate < 0 {
				currentRate = 0
			}

			// Calculate the number of requests for this interval
			requestsThisInterval := currentRate * interval.Seconds()
			totalRequestsFloat := requestsThisInterval + fractionalRequests
			numRequests := int(math.Floor(totalRequestsFloat))
			fractionalRequests = totalRequestsFloat - float64(numRequests)

			// Send the requests by adding to the request channel
			for i := 0; i < numRequests; i++ {
				select {
				case requestChan <- struct{}{}:
					// Request added to the channel
				case <-ctx.Done():
					// Context canceled while sending
					close(requestChan)
					wg.Wait()
					break loop // Exit the loop to proceed to CSV writing
				}
			}

			// Update the total number of requests
			totalRequests += numRequests

			// Log the interval data
			intervalData := IntervalData{
				ElapsedTimeMs: elapsed * 1000, // Convert seconds to milliseconds
				NumRequests:   numRequests,
			}

			// Protect concurrent access to the intervals slice
			intervalsMutex.Lock()
			intervals = append(intervals, intervalData)
			intervalsMutex.Unlock()
		}
	}

	// Write interval data to CSV
	outputFileName := "output.csv"

	// Create the CSV file
	file, err := os.Create(outputFileName)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Initialize CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV headers
	err = writer.Write([]string{"ElapsedTimeMs", "NumRequests"})
	if err != nil {
		return fmt.Errorf("failed to write CSV headers: %w", err)
	}

	// Write interval data
	intervalsMutex.Lock()
	defer intervalsMutex.Unlock()
	for _, data := range intervals {
		record := []string{
			fmt.Sprintf("%.0f", data.ElapsedTimeMs),
			fmt.Sprintf("%d", data.NumRequests),
		}
		err := writer.Write(record)
		if err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	fmt.Printf("Interval data has been written to %s\n", outputFileName)
	fmt.Printf("Total number of requests sent: %d\n", totalRequests)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <config_file>")
		os.Exit(1)
	}

	configFile := os.Args[1]
	config, err := loadConfig(configFile)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Destination: %s\nDuration: %f seconds\n", config.Destination, config.Duration)

	// Start sending requests
	fmt.Println("Starting to send requests...")
	err = sendRequests(config)
	if err != nil {
		fmt.Printf("Error sending requests: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Finished sending requests.")
}
