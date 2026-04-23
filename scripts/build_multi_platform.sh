#!/usr/bin/env bash
set -euo pipefail

TARGETS=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
  "windows amd64"
)

for target in "${TARGETS[@]}"; do
  IFS=' ' read -r GOOS GOARCH <<<"$target"
  echo "==> building packages for ${GOOS}/${GOARCH}"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build ./...
done

echo "all targets built successfully"
