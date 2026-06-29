#!/usr/bin/env sh
set -eu

BUILDX_BUILDER=${BUILDX_BUILDER:-podcast-rss-builder}

if docker buildx inspect "$BUILDX_BUILDER" >/dev/null 2>&1; then
  docker buildx use "$BUILDX_BUILDER" >/dev/null
else
  docker buildx create --name "$BUILDX_BUILDER" --driver docker-container --use >/dev/null
fi

docker buildx inspect "$BUILDX_BUILDER" --bootstrap >/dev/null
printf '%s\n' "$BUILDX_BUILDER"
