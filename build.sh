#!/bin/bash
set -e

NAME="skillman"
ROOT="$(cd "$(dirname "$0")" && pwd)"
DIR="$ROOT/build"

rm -rf "$DIR"
mkdir -p "$DIR"

build() {
  local os=$1 arch=$2 ext=$3
  local output="$DIR/${NAME}-${os}-${arch}${ext}"
  echo "Building ${os}/${arch}..."
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -ldflags="-s -w" -o "$output" "$ROOT"
}

build darwin  arm64  ""
build darwin  amd64  ""
build linux   arm64  ""
build linux   amd64  ""

echo ""
ls -lh "$DIR"
