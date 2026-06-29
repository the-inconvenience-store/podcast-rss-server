#!/usr/bin/env sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: scripts/validate-feed.sh <public-feed-url>" >&2
  exit 2
fi

FEED_URL="$1"
VALIDATOR_URL="https://www.rssboard.org/rss-validator/check.cgi?url=${FEED_URL}"

curl -fsSL "$VALIDATOR_URL" | tee /tmp/podcast-rss-validator.html
if grep -qi "This is a valid RSS feed" /tmp/podcast-rss-validator.html; then
  echo "feed validator reported zero RSS errors"
else
  echo "feed validator did not report a clean feed" >&2
  exit 1
fi
