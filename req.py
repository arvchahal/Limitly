import requests
import threading
import argparse
import time

# Function to send a GET request
def send_request(url):
    try:
        response = requests.get(url)
        print(f"Status Code: {response.status_code}, Content: {response.text[:50]}")  # Print status and snippet
    except requests.exceptions.RequestException as e:
        print(f"Request failed: {e}")

# Worker thread to send requests at a controlled rate
def worker(url, interval, num_requests):
    for _ in range(num_requests):
        send_request(url)
        time.sleep(interval)

def main():
    # Parse command-line arguments
    parser = argparse.ArgumentParser(description="Send configurable HTTP requests.")
    parser.add_argument("url", help="Target URL for HTTP requests.")
    parser.add_argument("requests", type=int, help="Total number of requests to send.")
    parser.add_argument("time", type=float, help="Total time (in seconds) to send all requests.")
    args = parser.parse_args()

    url = args.url
    total_requests = args.requests
    total_time = args.time

    # Calculate the interval between requests
    interval = total_time / total_requests

    # Use threading to manage request sending
    threads = []
    for _ in range(total_requests):
        thread = threading.Thread(target=worker, args=(url, interval, 1))
        threads.append(thread)
        thread.start()

    # Wait for all threads to complete
    for thread in threads:
        thread.join()

if __name__ == "__main__":
    main()
