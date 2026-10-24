#!/usr/bin/env bash
# Cross-compiles one binary per release target into dist/.
# Usage: scripts/release.sh <name> [target...]

set -euo pipefail
cd "$(dirname "$0")/.."

go_platform() {
    case "$1" in
        windows-arm64)       echo "windows arm64" ;;
        windows-x86)         echo "windows 386" ;;
        windows-x64)         echo "windows amd64" ;;
        linux-arm32)         echo "linux arm" ;;
        linux-arm64)         echo "linux arm64" ;;
        linux-x86)           echo "linux 386" ;;
        linux-x64)           echo "linux amd64" ;;
        macOS-intel)         echo "darwin amd64" ;;
        macOS-apple-silicon) echo "darwin arm64" ;;
        *) echo "Unknown release target: $1" >&2; return 1 ;;
    esac
}

name="$1"
shift

rm -rf dist
mkdir -p dist
for target in "$@"; do
    read -r goos goarch <<< "$(go_platform "$target")"
    suffix=""
    if [ "$goos" = "windows" ]; then
        suffix=".exe"
    fi
    echo "Building $target"
    # GOARM only affects the 32-bit arm target, where 7 covers every current board
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" GOARM=7 \
        go build -trimpath -o "dist/$name-$target$suffix" ./cmd
done
