#!/usr/bin/env sh
set -eu

BUMP=${1:-patch}
VERSION_FILE=${VERSION_FILE:-VERSION}

VERSION=$(tr -d '[:space:]' < "$VERSION_FILE")
case "$VERSION" in
  *.*.*) ;;
  *)
    echo "VERSION must be semantic version major.minor.patch, got: $VERSION" >&2
    exit 2
    ;;
esac

IFS=.
set -- $VERSION
MAJOR=$1
MINOR=$2
PATCH=$3
unset IFS

case "$BUMP" in
  major)
    MAJOR=$((MAJOR + 1))
    MINOR=0
    PATCH=0
    ;;
  minor)
    MINOR=$((MINOR + 1))
    PATCH=0
    ;;
  patch)
    PATCH=$((PATCH + 1))
    ;;
  *)
    echo "usage: scripts/next-version.sh [major|minor|patch]" >&2
    exit 2
    ;;
esac

printf '%s.%s.%s\n' "$MAJOR" "$MINOR" "$PATCH"
