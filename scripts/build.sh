#!/usr/bin/env bash
# Cross-compile confluence-cli for all supported platforms into dist/.
set -euo pipefail

cd "$(dirname "$0")/.."

BINARY="confluence-cli"
PKG="github.com/angelmsger/confluence-cli"
CONSTANTS="${PKG}/pkg/constants"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo none)}"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

LDFLAGS="-s -w \
  -X ${CONSTANTS}.Version=${VERSION} \
  -X ${CONSTANTS}.Commit=${COMMIT} \
  -X ${CONSTANTS}.BuildTime=${BUILD_TIME}"

PLATFORMS=(
  "darwin/amd64" "darwin/arm64"
  "linux/amd64"  "linux/arm64"
  "windows/amd64" "windows/arm64"
)

mkdir -p dist
for platform in "${PLATFORMS[@]}"; do
  GOOS="${platform%/*}"
  GOARCH="${platform#*/}"
  out="dist/${BINARY}-${GOOS}-${GOARCH}"
  [ "$GOOS" = "windows" ] && out="${out}.exe"
  echo "building ${out}"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
    go build -trimpath -ldflags "$LDFLAGS" -o "$out" ./cmd/confluence-cli
done
echo "done -> dist/"
