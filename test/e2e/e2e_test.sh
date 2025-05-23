#!/bin/bash
set -e

echo "Starting E2E test..."

# 1. Set up a mock upstream server
echo "Setting up mock upstream server..."
# Ensure /tmp/upstream_received.txt is clean before test
rm -f /tmp/upstream_received.txt
# Start nc: send a 200 OK response, then write client's request to /tmp/upstream_received.txt
(echo -ne "HTTP/1.1 200 OK\r\nContent-Length: 0\r\nConnection: close\r\n\r\n" | nc -l -p 8081 > /tmp/upstream_received.txt) &
NC_PID=$!
echo "Mock upstream server started with PID: $NC_PID"
sleep 2 # Wait for nc to start

# 2. Start gitwebhookproxy
# Ensure gitwebhookproxy binary exists
GWP_BINARY="./gitwebhookproxy"
if [ ! -f "$GWP_BINARY" ]; then
    echo "gitwebhookproxy binary not found at $GWP_BINARY. Attempting to build..."
    # Assuming a simple go build command; adjust if your build process is different
    # This requires Go to be installed in the environment where the script runs.
    if command -v go &> /dev/null; then
        go build -o "$GWP_BINARY" ../../gitwebhookproxy.go
        if [ $? -ne 0 ]; then
            echo "Failed to build gitwebhookproxy. Exiting."
            # Clean up nc before exiting
            kill $NC_PID 2>/dev/null || true
            wait $NC_PID 2>/dev/null || true
            exit 1
        fi
        echo "gitwebhookproxy built successfully."
    else
        echo "Go is not installed. Cannot build gitwebhookproxy. Exiting."
        # Clean up nc before exiting
        kill $NC_PID 2>/dev/null || true
        wait $NC_PID 2>/dev/null || true
        exit 1
    fi
elif [ ! -x "$GWP_BINARY" ]; then
    echo "$GWP_BINARY is not executable. Attempting to make it executable..."
    chmod +x "$GWP_BINARY"
    if [ $? -ne 0 ]; then
        echo "Failed to make $GWP_BINARY executable. Exiting."
        # Clean up nc before exiting
        kill $NC_PID 2>/dev/null || true
        wait $NC_PID 2>/dev/null || true
        exit 1
    fi
fi


echo "Starting gitwebhookproxy..."
"$GWP_BINARY" -listen :8080 -upstreamURL http://localhost:8081 -allowedPaths /testwebhook &
GWP_PID=$!
echo "gitwebhookproxy started with PID: $GWP_PID"
sleep 2 # Wait for proxy to start

# 3. Send a test webhook
echo "Sending test webhook..."
HTTP_STATUS_CODE=$(curl -X POST \
  -d '{"test": "data"}' \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature: sha1=testsignature" \
  -H "X-GitHub-Event: testevent" \
  -H "X-GitHub-Delivery: testdeliveryid" \
  http://localhost:8080/testwebhook --silent --output /dev/null -w "%{http_code}")
echo "Received HTTP status code: $HTTP_STATUS_CODE"

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    echo "Killing gitwebhookproxy (PID: $GWP_PID)..."
    kill $GWP_PID
    wait $GWP_PID 2>/dev/null || true # Wait for process to terminate, ignore error if already dead
    echo "Killing mock upstream server (PID: $NC_PID)..."
    kill $NC_PID 2>/dev/null || true
    wait $NC_PID 2>/dev/null || true # Wait for process to terminate, ignore error if already dead
    echo "Removing /tmp/upstream_received.txt..."
    rm -f /tmp/upstream_received.txt
    echo "Cleanup finished."
}

# 4. Verify results
echo "Verifying results..."
if [[ "$HTTP_STATUS_CODE" -lt 200 || "$HTTP_STATUS_CODE" -ge 300 ]]; then
    echo "Error: Expected HTTP status code 2xx, but got $HTTP_STATUS_CODE."
    cleanup
    exit 1
fi
echo "HTTP status code OK."

# Wait for upstream_received.txt to be populated
MAX_WAIT_SECONDS=10
WAIT_INTERVAL_SECONDS=1
ELAPSED_SECONDS=0
FILE_POPULATED=false

echo "Waiting for mock upstream server to receive data..."
while [ $ELAPSED_SECONDS -lt $MAX_WAIT_SECONDS ]; do
    if [ -s /tmp/upstream_received.txt ]; then
        FILE_POPULATED=true
        break
    fi
    sleep $WAIT_INTERVAL_SECONDS
    ELAPSED_SECONDS=$((ELAPSED_SECONDS + WAIT_INTERVAL_SECONDS))
    echo "Waited $ELAPSED_SECONDS seconds..."
done

if [ "$FILE_POPULATED" = false ]; then
    echo "Error: /tmp/upstream_received.txt is empty or does not exist after $MAX_WAIT_SECONDS seconds. Mock upstream server received no data."
    cleanup
    exit 1
fi
echo "/tmp/upstream_received.txt is not empty."

# More robust check for content
EXPECTED_CONTENT='{"test": "data"}'
# The actual content received by nc might include HTTP headers.
# We check if the expected JSON payload is present.
if ! grep -q "$EXPECTED_CONTENT" /tmp/upstream_received.txt; then
    echo "Error: /tmp/upstream_received.txt does not contain the expected content '$EXPECTED_CONTENT'."
    echo "Actual content:"
    cat /tmp/upstream_received.txt
    cleanup
    exit 1
fi
echo "Content of /tmp/upstream_received.txt is correct."

# 5. Cleanup (already defined in function, call it)
cleanup

# 6. Success
echo "E2E test successful!"
exit 0
