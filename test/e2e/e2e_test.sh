#!/bin/bash
set -e

echo "Starting E2E test..."

# --- Configuration ---
# Assuming this script is in test/e2e/ and gitwebhookproxy is in the repo root
GWP_BINARY="../../gitwebhookproxy"
LOG_DIR="/tmp/e2e_logs"
mkdir -p "$LOG_DIR" # Ensure log directory exists

# --- PID Management & Cleanup ---
ALL_PIDS=() # Array to store all background PIDs

cleanup() {
    echo "Cleaning up..."
    for pid in "${ALL_PIDS[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then # Check if process exists
            echo "Killing process (PID: $pid)..."
            kill "$pid"
            wait "$pid" 2>/dev/null || true # Wait for process to terminate
        else
            echo "Process (PID: $pid) already terminated."
        fi
    done
    # Remove mock upstream output files if they exist
    rm -f "$LOG_DIR/e2e_upstream1_received.txt"
    rm -f "$LOG_DIR/e2e_upstream2_received.txt"
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

# --- Mock Upstream Function ---
start_mock_upstream() {
    local port="$1"
    local output_file="$2"
    
    echo "Starting mock upstream on port $port, outputting to $output_file..."
    # Using a subshell to redirect output of nc to the file even if nc exits quickly
    # Adding a small sleep to ensure nc has time to start listening
    ( (nc -l -p "$port" > "$output_file" 2> "$LOG_DIR/nc_${port}_stderr.log") & )
    NC_PID=$!
    sleep 0.5 # Give nc a moment to start
    if ! kill -0 $NC_PID 2>/dev/null; then
        echo "ERROR: nc failed to start on port $port. Check $LOG_DIR/nc_${port}_stderr.log"
        exit 1
    fi
    ALL_PIDS+=("$NC_PID")
    echo "Mock upstream started on port $port, PID: $NC_PID, output file: $output_file"
}

# --- Helper for Verification ---
verify_upstream_output() {
    local output_file="$1"
    local expected_payload="$2"
    local max_wait_seconds=5
    local wait_interval_seconds=0.5
    local elapsed_seconds=0
    local file_populated=false

    echo "Waiting for mock upstream server to receive data at $output_file..."
    while [ "$elapsed_seconds" -lt "$max_wait_seconds" ]; do
        if [ -s "$output_file" ]; then # Check if file is not empty
            # Check if the content contains the payload. nc might receive headers too.
            if grep -q "$expected_payload" "$output_file"; then
                file_populated=true
                break
            fi
        fi
        sleep $wait_interval_seconds
        elapsed_seconds=$(awk "BEGIN {print $elapsed_seconds + $wait_interval_seconds}")
    done

    if [ "$file_populated" = false ]; then
        echo "Error: $output_file did not contain expected payload '$expected_payload' after $max_wait_seconds seconds."
        echo "Actual content of $output_file:"
        cat "$output_file" || echo "$output_file is empty or does not exist."
        exit 1
    fi
    echo "$output_file received expected content."
}

# --- Test Case 1: Legacy -upstreamURL ---
echo "--- Test Case 1: Legacy -upstreamURL ---"
UPSTREAM1_PORT=8091
UPSTREAM1_OUTPUT_FILE="$LOG_DIR/e2e_upstream1_received.txt"
GWP_LOG1="$LOG_DIR/gwp1.log"
rm -f "$UPSTREAM1_OUTPUT_FILE" # Clear previous output

start_mock_upstream "$UPSTREAM1_PORT" "$UPSTREAM1_OUTPUT_FILE"
NC_PID_1=$ALL_PIDS[${#ALL_PIDS[@]}-1] # Get last added PID

echo "Starting gitwebhookproxy for Test Case 1..."
"$GWP_BINARY" -listen :8080 -upstreamURL "http://localhost:$UPSTREAM1_PORT" -allowedPaths /testwebhook1 > "$GWP_LOG1" 2>&1 &
GWP_PID_1=$!
ALL_PIDS+=("$GWP_PID_1")
echo "gitwebhookproxy (Test Case 1) started with PID: $GWP_PID_1, logging to $GWP_LOG1"
sleep 1 # Wait for proxy to start

echo "Sending test webhook for Test Case 1..."
HTTP_STATUS_CODE_1=$(curl -X POST \
  -d '{"test": "data1"}' \
  -H "Content-Type: application/json" \
  http://localhost:8080/testwebhook1 --silent --output /dev/null -w "%{http_code}")
echo "Received HTTP status code (Test Case 1): $HTTP_STATUS_CODE_1"

if [[ "$HTTP_STATUS_CODE_1" -lt 200 || "$HTTP_STATUS_CODE_1" -ge 300 ]]; then
    echo "Error (Test Case 1): Expected HTTP status code 2xx from gitwebhookproxy, but got $HTTP_STATUS_CODE_1."
    echo "GWP logs ($GWP_LOG1):"
    cat "$GWP_LOG1"
    exit 1
fi
echo "HTTP status code OK for Test Case 1."

verify_upstream_output "$UPSTREAM1_OUTPUT_FILE" '{"test": "data1"}'

echo "Stopping gitwebhookproxy (Test Case 1)..."
kill $GWP_PID_1 && wait $GWP_PID_1 2>/dev/null || true
# NC_PID_1 will be cleaned by trap or specific cleanup if needed earlier
echo "--- Test Case 1 Passed ---"
echo ""

# --- Test Case 2: New -upstreamURLs (Single URL) ---
echo "--- Test Case 2: New -upstreamURLs (Single URL) ---"
UPSTREAM2_PORT=8092
UPSTREAM2_OUTPUT_FILE="$LOG_DIR/e2e_upstream2_received.txt"
GWP_LOG2="$LOG_DIR/gwp2.log"
rm -f "$UPSTREAM2_OUTPUT_FILE" # Clear previous output

start_mock_upstream "$UPSTREAM2_PORT" "$UPSTREAM2_OUTPUT_FILE"
# NC_PID_2 is added to ALL_PIDS by start_mock_upstream

echo "Starting gitwebhookproxy for Test Case 2..."
"$GWP_BINARY" -listen :8081 -upstreamURLs "http://localhost:$UPSTREAM2_PORT" -allowedPaths /testwebhook2 > "$GWP_LOG2" 2>&1 &
GWP_PID_2=$!
ALL_PIDS+=("$GWP_PID_2")
echo "gitwebhookproxy (Test Case 2) started with PID: $GWP_PID_2 on port 8081, logging to $GWP_LOG2"
sleep 1 # Wait for proxy to start

echo "Sending test webhook for Test Case 2..."
HTTP_STATUS_CODE_2=$(curl -X POST \
  -d '{"test": "data2"}' \
  -H "Content-Type: application/json" \
  http://localhost:8081/testwebhook2 --silent --output /dev/null -w "%{http_code}")
echo "Received HTTP status code (Test Case 2): $HTTP_STATUS_CODE_2"

if [[ "$HTTP_STATUS_CODE_2" -lt 200 || "$HTTP_STATUS_CODE_2" -ge 300 ]]; then
    echo "Error (Test Case 2): Expected HTTP status code 2xx from gitwebhookproxy, but got $HTTP_STATUS_CODE_2."
    echo "GWP logs ($GWP_LOG2):"
    cat "$GWP_LOG2"
    exit 1
fi
echo "HTTP status code OK for Test Case 2."

verify_upstream_output "$UPSTREAM2_OUTPUT_FILE" '{"test": "data2"}'

echo "Stopping gitwebhookproxy (Test Case 2)..."
kill $GWP_PID_2 && wait $GWP_PID_2 2>/dev/null || true
echo "--- Test Case 2 Passed ---"
echo ""


# --- Test Case 3: New -upstreamURLs (Multiple URLs) ---
echo "--- Test Case 3: New -upstreamURLs (Multiple URLs) ---"
UPSTREAM3A_PORT=8093
UPSTREAM3A_OUTPUT_FILE="$LOG_DIR/e2e_multi1_received.txt"
UPSTREAM3B_PORT=8094
UPSTREAM3B_OUTPUT_FILE="$LOG_DIR/e2e_multi2_received.txt"
GWP_LOG3="$LOG_DIR/gwp3.log"
rm -f "$UPSTREAM3A_OUTPUT_FILE" "$UPSTREAM3B_OUTPUT_FILE"

start_mock_upstream "$UPSTREAM3A_PORT" "$UPSTREAM3A_OUTPUT_FILE"
start_mock_upstream "$UPSTREAM3B_PORT" "$UPSTREAM3B_OUTPUT_FILE"

echo "Starting gitwebhookproxy for Test Case 3..."
"$GWP_BINARY" -listen :8082 -upstreamURLs "http://localhost:$UPSTREAM3A_PORT,http://localhost:$UPSTREAM3B_PORT" -allowedPaths /testwebhook3 > "$GWP_LOG3" 2>&1 &
GWP_PID_3=$!
ALL_PIDS+=("$GWP_PID_3")
echo "gitwebhookproxy (Test Case 3) started with PID: $GWP_PID_3 on port 8082, logging to $GWP_LOG3"
sleep 1

echo "Sending test webhook for Test Case 3..."
HTTP_STATUS_CODE_3=$(curl -X POST \
  -d '{"test": "data3"}' \
  -H "Content-Type: application/json" \
  http://localhost:8082/testwebhook3 --silent --output /dev/null -w "%{http_code}")
echo "Received HTTP status code (Test Case 3): $HTTP_STATUS_CODE_3"

if [[ "$HTTP_STATUS_CODE_3" -lt 200 || "$HTTP_STATUS_CODE_3" -ge 300 ]]; then
    echo "Error (Test Case 3): Expected HTTP status code 2xx, but got $HTTP_STATUS_CODE_3."
    echo "GWP logs ($GWP_LOG3):"
    cat "$GWP_LOG3"
    exit 1
fi
echo "HTTP status code OK for Test Case 3."

verify_upstream_output "$UPSTREAM3A_OUTPUT_FILE" '{"test": "data3"}'
verify_upstream_output "$UPSTREAM3B_OUTPUT_FILE" '{"test": "data3"}'

echo "Stopping gitwebhookproxy (Test Case 3)..."
kill $GWP_PID_3 && wait $GWP_PID_3 2>/dev/null || true
echo "--- Test Case 3 Passed ---"
echo ""


# --- Test Case 4: Both -upstreamURL and -upstreamURLs (Distinct URLs) ---
echo "--- Test Case 4: Both -upstreamURL and -upstreamURLs (Distinct URLs) ---"
UPSTREAM4A_PORT=8095
UPSTREAM4A_OUTPUT_FILE="$LOG_DIR/e2e_combo1_received.txt"
UPSTREAM4B_PORT=8096
UPSTREAM4B_OUTPUT_FILE="$LOG_DIR/e2e_combo2_received.txt"
GWP_LOG4="$LOG_DIR/gwp4.log"
rm -f "$UPSTREAM4A_OUTPUT_FILE" "$UPSTREAM4B_OUTPUT_FILE"

start_mock_upstream "$UPSTREAM4A_PORT" "$UPSTREAM4A_OUTPUT_FILE"
start_mock_upstream "$UPSTREAM4B_PORT" "$UPSTREAM4B_OUTPUT_FILE"

echo "Starting gitwebhookproxy for Test Case 4..."
"$GWP_BINARY" -listen :8083 \
  -upstreamURL "http://localhost:$UPSTREAM4A_PORT" \
  -upstreamURLs "http://localhost:$UPSTREAM4B_PORT" \
  -allowedPaths /testwebhook4 > "$GWP_LOG4" 2>&1 &
GWP_PID_4=$!
ALL_PIDS+=("$GWP_PID_4")
echo "gitwebhookproxy (Test Case 4) started with PID: $GWP_PID_4 on port 8083, logging to $GWP_LOG4"
sleep 1

echo "Sending test webhook for Test Case 4..."
HTTP_STATUS_CODE_4=$(curl -X POST \
  -d '{"test": "data4"}' \
  -H "Content-Type: application/json" \
  http://localhost:8083/testwebhook4 --silent --output /dev/null -w "%{http_code}")
echo "Received HTTP status code (Test Case 4): $HTTP_STATUS_CODE_4"

if [[ "$HTTP_STATUS_CODE_4" -lt 200 || "$HTTP_STATUS_CODE_4" -ge 300 ]]; then
    echo "Error (Test Case 4): Expected HTTP status code 2xx, but got $HTTP_STATUS_CODE_4."
    echo "GWP logs ($GWP_LOG4):"
    cat "$GWP_LOG4"
    exit 1
fi
echo "HTTP status code OK for Test Case 4."

verify_upstream_output "$UPSTREAM4A_OUTPUT_FILE" '{"test": "data4"}'
verify_upstream_output "$UPSTREAM4B_OUTPUT_FILE" '{"test": "data4"}'

echo "Stopping gitwebhookproxy (Test Case 4)..."
kill $GWP_PID_4 && wait $GWP_PID_4 2>/dev/null || true
echo "--- Test Case 4 Passed ---"
echo ""


# --- Test Case 5: Both -upstreamURL and -upstreamURLs (Overlapping/Duplicate URL) ---
echo "--- Test Case 5: Both -upstreamURL and -upstreamURLs (Overlapping/Duplicate URL) ---"
UPSTREAM5_PORT=8097
UPSTREAM5_OUTPUT_FILE="$LOG_DIR/e2e_overlap_received.txt"
GWP_LOG5="$LOG_DIR/gwp5.log"
rm -f "$UPSTREAM5_OUTPUT_FILE"

start_mock_upstream "$UPSTREAM5_PORT" "$UPSTREAM5_OUTPUT_FILE"

echo "Starting gitwebhookproxy for Test Case 5..."
"$GWP_BINARY" -listen :8084 \
  -upstreamURL "http://localhost:$UPSTREAM5_PORT" \
  -upstreamURLs "http://localhost:$UPSTREAM5_PORT" \
  -allowedPaths /testwebhook5 > "$GWP_LOG5" 2>&1 &
GWP_PID_5=$!
ALL_PIDS+=("$GWP_PID_5")
echo "gitwebhookproxy (Test Case 5) started with PID: $GWP_PID_5 on port 8084, logging to $GWP_LOG5"
sleep 1

echo "Sending test webhook for Test Case 5..."
HTTP_STATUS_CODE_5=$(curl -X POST \
  -d '{"test": "data5"}' \
  -H "Content-Type: application/json" \
  http://localhost:8084/testwebhook5 --silent --output /dev/null -w "%{http_code}")
echo "Received HTTP status code (Test Case 5): $HTTP_STATUS_CODE_5"

if [[ "$HTTP_STATUS_CODE_5" -lt 200 || "$HTTP_STATUS_CODE_5" -ge 300 ]]; then
    echo "Error (Test Case 5): Expected HTTP status code 2xx, but got $HTTP_STATUS_CODE_5."
    echo "GWP logs ($GWP_LOG5):"
    cat "$GWP_LOG5"
    exit 1
fi
echo "HTTP status code OK for Test Case 5."

verify_upstream_output "$UPSTREAM5_OUTPUT_FILE" '{"test": "data5"}'
# For verifying "received only once", we rely on the fact that `verify_upstream_output`
# checks for the presence of the payload. If it were sent multiple times in a simple way,
# the payload might appear multiple times. A more strict check could be `grep -c '{"test": "data5"}' $UPSTREAM5_OUTPUT_FILE`
# and ensure the count is 1. However, nc includes HTTP headers, so the payload itself
# should appear once per request. The deduplication in GWP should mean only one request hits the upstream.
# We can manually inspect logs if needed, but for automation, presence is the main check.
# Checking the GWP log for the "Consolidated upstream URLs" line can also be informative.
echo "Verifying GWP logs for Test Case 5 to check for deduplication..."
if grep -q "Consolidated upstream URLs: \[http://localhost:$UPSTREAM5_PORT\]" "$GWP_LOG5"; then
    echo "Deduplication in GWP logs confirmed for Test Case 5."
else
    echo "Error (Test Case 5): Deduplication not evident in GWP logs."
    echo "GWP logs ($GWP_LOG5):"
    cat "$GWP_LOG5"
    exit 1
fi


echo "Stopping gitwebhookproxy (Test Case 5)..."
kill $GWP_PID_5 && wait $GWP_PID_5 2>/dev/null || true
echo "--- Test Case 5 Passed ---"
echo ""


# --- Success ---
echo "All E2E tests successful!"
exit 0
