#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

printf '1.2.3\n' > "$TMP_DIR/VERSION"

NEXT_PATCH=$(VERSION_FILE="$TMP_DIR/VERSION" "$ROOT_DIR/scripts/next-version.sh" patch)
[ "$NEXT_PATCH" = "1.2.4" ] || {
  echo "patch bump = $NEXT_PATCH, want 1.2.4" >&2
  exit 1
}

NEXT_MINOR=$(VERSION_FILE="$TMP_DIR/VERSION" "$ROOT_DIR/scripts/next-version.sh" minor)
[ "$NEXT_MINOR" = "1.3.0" ] || {
  echo "minor bump = $NEXT_MINOR, want 1.3.0" >&2
  exit 1
}

NEXT_MAJOR=$(VERSION_FILE="$TMP_DIR/VERSION" "$ROOT_DIR/scripts/next-version.sh" major)
[ "$NEXT_MAJOR" = "2.0.0" ] || {
  echo "major bump = $NEXT_MAJOR, want 2.0.0" >&2
  exit 1
}

PLAN=$(VERSION_FILE="$TMP_DIR/VERSION" DOCKERHUB_REPO="samstevens/podcast-rss" "$ROOT_DIR/scripts/docker-publish.sh" --print)
echo "$PLAN" | grep -F "docker buildx build" >/dev/null
echo "$PLAN" | grep -F "samstevens/podcast-rss:1.2.3" >/dev/null
echo "$PLAN" | grep -F "samstevens/podcast-rss:latest" >/dev/null
echo "$PLAN" | grep -F "linux/amd64,linux/arm64" >/dev/null
