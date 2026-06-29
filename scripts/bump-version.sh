#!/usr/bin/env sh
set -eu

VERSION_FILE=${VERSION_FILE:-VERSION}
NEXT_VERSION=$(VERSION_FILE="$VERSION_FILE" "$(dirname -- "$0")/next-version.sh" "${1:-patch}")
printf '%s\n' "$NEXT_VERSION" > "$VERSION_FILE"
printf '%s\n' "$NEXT_VERSION"
