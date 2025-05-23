#!/bin/bash
set -e

echo "Starting E2E test..."

# --- Configuration ---
# Assuming this script is in test/e2e/ and gitwebhookproxy is in the repo root
GWP_BINARY="../../gitwebhookproxy"
LOG_DIR="/tmp/e2e_gwp_logs" # Directory for GWP logs
mkdir -p "$LOG_DIR"

# --- PID Management & Cleanup ---
ALL_GWP_PIDS=() # Array to store all GWP background PIDs

cleanup() {
    echo "Cleaning up GWP instances..."
    for pid in "${ALL_GWP_PIDS[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then # Check if process exists
            echo "Killing gitwebhookproxy (PID: $pid)..."
            kill "$pid"
            wait "$pid" 2>/dev/null || true # Wait for process to terminate
        else
            echo "gitwebhookproxy (PID: $pid) already terminated or PID not captured."
        fi
    done
    echo "Cleanup finished."
}
trap cleanup EXIT

# --- Binary Check & Build ---
if [ ! -f "$GWP_BINARY" ]; then
    echo "gitwebhookproxy binary not found at $GWP_BINARY. Attempting to build..."
    if command -v go &> /dev/null; then
        (cd ../../ && go build -o "$GWP_BINARY" .) # Build in repo root
        if [ $? -ne 0 ]; then
            echo "Failed to build gitwebhookproxy. Exiting."
            exit 1
        fi
        echo "gitwebhookproxy built successfully."
    else
        echo "Go is not installed. Cannot build gitwebhookproxy. Exiting."
        exit 1
    fi
elif [ ! -x "$GWP_BINARY" ]; then
    echo "$GWP_BINARY is not executable. Attempting to make it executable..."
    chmod +x "$GWP_BINARY"
    if [ $? -ne 0 ]; then
        echo "Failed to make $GWP_BINARY executable. Exiting."
        exit 1
    fi
fi

# --- Helper function to run a test case ---
run_test_case() {
    local test_name="$1"
    local gwp_args="$2"
    local curl_path="$3"
    local expected_echo_servers_msg="$4"
    local gwp_log_file="$5" # Optional: log file for GWP for specific checks

    echo "--- Test Case: $test_name ---"
    
    local current_gwp_pid
    
    echo "Starting gitwebhookproxy with args: $gwp_args"
    # shellcheck disable=SC2086
    "$GWP_BINARY" -listen :8080 $gwp_args > "$gwp_log_file" 2>&1 &
    current_gwp_pid=$!
    ALL_GWP_PIDS+=("$current_gwp_pid")
    echo "gitwebhookproxy started with PID: $current_gwp_pid, logging to $gwp_log_file"
    sleep 2 # Wait for proxy to start

    echo "Sending test webhook to $curl_path..."
    HTTP_STATUS_CODE=$(curl -X POST \
      -d '{"test": "data"}' \
      -H "Content-Type: application/json" \
      -H "X-Hub-Signature: sha1=testsignature" \
      -H "X-GitHub-Event: testevent" \
      -H "X-GitHub-Delivery: testdeliveryid" \
      "http://localhost:8080$curl_path" --silent --output /dev/null -w "%{http_code}")
    echo "Received HTTP status code: $HTTP_STATUS_CODE"

    if [[ "$HTTP_STATUS_CODE" -lt 200 || "$HTTP_STATUS_CODE" -ge 300 ]]; then
        echo "Error ($test_name): Expected HTTP status code 2xx from gitwebhookproxy, but got $HTTP_STATUS_CODE."
        echo "GWP logs ($gwp_log_file):"
        cat "$gwp_log_file"
        exit 1
    fi
    echo "HTTP status code OK for $test_name."
    echo "$expected_echo_servers_msg"

    # Specific checks for Test Case 5 (Deduplication)
    if [[ "$test_name" == "Both -upstreamURL and -upstreamURLs (Overlapping/Duplicate URL) - Deduplication Test" ]]; then
        echo "Verifying GWP logs for deduplication..."
        # Simple grep for the line. Manual verification of uniqueness is implied if the line is complex.
        # A more robust check would be to count occurrences of each URL in the "Consolidated upstream URLs" log line.
        # Example: Consolidated upstream URLs: [http://localhost:8081 http://localhost:8082]
        if grep "Consolidated upstream URLs:" "$gwp_log_file"; then
            echo "Found 'Consolidated upstream URLs' log line. Please verify for correct deduplication (e.g., http://localhost:8081 should appear once)."
            # Attempting a basic automated check for the specific case
            # This checks if "http://localhost:8081" appears exactly once in the line containing "Consolidated upstream URLs"
            # and "http://localhost:8082" also appears.
            consolidated_line=$(grep "Consolidated upstream URLs:" "$gwp_log_file")
            count_8081=$(echo "$consolidated_line" | grep -o "http://localhost:8081" | wc -l)
            count_8082=$(echo "$consolidated_line" | grep -o "http://localhost:8082" | wc -l)

            if [ "$count_8081" -eq 1 ] && [ "$count_8082" -eq 1 ]; then
                echo "Automated deduplication check passed: http://localhost:8081 appears once, http://localhost:8082 appears once in consolidated list."
            else
                echo "ERROR: Automated deduplication check failed. Count for 8081: $count_8081 (expected 1), Count for 8082: $count_8082 (expected 1)."
                echo "Full consolidated line: $consolidated_line"
                exit 1
            fi
        else
            echo "ERROR: 'Consolidated upstream URLs' log line not found in $gwp_log_file."
            exit 1
        fi
    fi

    echo "Stopping gitwebhookproxy (PID: $current_gwp_pid) for $test_name..."
    kill "$current_gwp_pid"
    wait "$current_gwp_pid" 2>/dev/null || true
    # Remove from ALL_GWP_PIDS as it's handled, though trap cleanup is the main safety net
    ALL_GWP_PIDS=("${ALL_GWP_PIDS[@]/$current_gwp_pid}") 
    sleep 2 # Give port time to be freed
    echo "--- $test_name Passed ---"
    echo ""
}

# --- Run Test Cases ---
# Test Case 1
run_test_case \
  "Legacy -upstreamURL only" \
  "-upstreamURL http://localhost:8081 -allowedPaths /testwebhook" \
  "/testwebhook" \
  "Test Case 1: Check logs for echo-server on port 8081." \
  "$LOG_DIR/gwp_tc1.log"

# Test Case 2
run_test_case \
  "New -upstreamURLs (single URL)" \
  "-upstreamURLs http://localhost:8082 -allowedPaths /testwebhook" \
  "/testwebhook" \
  "Test Case 2: Check logs for echo-server on port 8082." \
  "$LOG_DIR/gwp_tc2.log"

# Test Case 3
run_test_case \
  "New -upstreamURLs (multiple URLs)" \
  "-upstreamURLs http://localhost:8081,http://localhost:8082 -allowedPaths /testwebhook" \
  "/testwebhook" \
  "Test Case 3: Check logs for echo-servers on ports 8081 and 8082." \
  "$LOG_DIR/gwp_tc3.log"

# Test Case 4
run_test_case \
  "Both -upstreamURL and -upstreamURLs (distinct URLs)" \
  "-upstreamURL http://localhost:8081 -upstreamURLs http://localhost:8083 -allowedPaths /testwebhook" \
  "/testwebhook" \
  "Test Case 4: Check logs for echo-servers on ports 8081 and 8083." \
  "$LOG_DIR/gwp_tc4.log"

# Test Case 5
run_test_case \
  "Both -upstreamURL and -upstreamURLs (Overlapping/Duplicate URL) - Deduplication Test" \
  "-upstreamURL http://localhost:8081 -upstreamURLs http://localhost:8081,http://localhost:8082 -allowedPaths /testwebhook" \
  "/testwebhook" \
  "Test Case 5: Check logs for echo-servers on ports 8081 and 8082." \
  "$LOG_DIR/gwp_tc5_dedup.log"

# --- Success ---
echo "All E2E tests passed!"
exit 0
