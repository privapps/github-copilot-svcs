#!/usr/bin/env bash

# Test vision capabilities through the proxy
# Usage: ./test_vision_proxy.sh [image_path]

set -euo pipefail

PROXY_URL="${PROXY_URL:-http://localhost:8081}"
IMAGE_PATH="${1:-dog.jpeg}"
PROMPT="${2:-Describe the attached image in detail.}"

if [[ ! -f "$IMAGE_PATH" ]]; then
  echo "Image '$IMAGE_PATH' does not exist." >&2
  exit 1
fi

# Get token from config
TOKEN=no_token

echo "Testing vision through proxy at $PROXY_URL..."
echo "Image: $IMAGE_PATH"
echo "Token: ${TOKEN:0:10}..."

# Encode image
IMAGE_MIME="$(file --brief --mime-type "$IMAGE_PATH")"
IMAGE_BASE64="$(
  python3 - "$IMAGE_PATH" <<'PY'
import base64, sys
with open(sys.argv[1], "rb") as f:
    print(base64.b64encode(f.read()).decode("ascii"))
PY
)"

# Create request payload
REQUEST_FILE="$(mktemp)"
trap 'rm -f "$REQUEST_FILE"' EXIT

jq -n \
  --arg model "gpt-4o" \
  --arg prompt "$PROMPT" \
  --arg image "data:$IMAGE_MIME;base64,$IMAGE_BASE64" \
  '{
    model: $model,
    messages: [
      {
        role: "user",
        content: [
          {type: "text", text: $prompt},
          {type: "image_url", image_url: {url: $image}}
        ]
      }
    ],
    max_tokens: 500
  }' > "$REQUEST_FILE"

echo "Request payload size: $(stat -f%z "$REQUEST_FILE") bytes"
echo "Sending request..."

# Send request through proxy (NOT directly to GitHub)
RESPONSE="$(curl -sS -X POST "$PROXY_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  --data-binary @"$REQUEST_FILE")"

echo ""
echo "Response:"
echo "$RESPONSE" | jq .

# Extract and display just the content
CONTENT=$(echo "$RESPONSE" | jq -r '.choices[0].message.content // "No content"')
echo ""
echo "=== AI Response ==="
echo "$CONTENT"
echo ""

# Check for errors
if echo "$RESPONSE" | jq -e '.error' > /dev/null 2>&1; then
  echo "ERROR in response!" >&2
  exit 1
fi

echo "✓ Vision test successful!"
