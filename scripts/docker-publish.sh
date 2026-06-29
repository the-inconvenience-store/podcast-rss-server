#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
VERSION_FILE=${VERSION_FILE:-"$ROOT_DIR/VERSION"}
IMAGE_NAME=${IMAGE_NAME:-podcast-rss}
PLATFORMS=${PLATFORMS:-linux/amd64,linux/arm64}
MODE=push

if [ "${1:-}" = "--print" ]; then
  MODE=print
  shift
elif [ "${1:-}" = "--load" ]; then
  MODE=load
  shift
elif [ "${1:-}" = "--push" ]; then
  MODE=push
  shift
fi

VERSION=${1:-$(tr -d '[:space:]' < "$VERSION_FILE")}

if [ -n "${DOCKERHUB_REPO:-}" ]; then
  REPO=$DOCKERHUB_REPO
elif [ -n "${DOCKERHUB_USERNAME:-}" ]; then
  REPO="${DOCKERHUB_USERNAME}/${IMAGE_NAME}"
else
  echo "set DOCKERHUB_REPO or DOCKERHUB_USERNAME" >&2
  exit 2
fi

case "$VERSION" in
  *.*.*) ;;
  *)
    echo "version must be semantic version major.minor.patch, got: $VERSION" >&2
    exit 2
    ;;
esac

IFS=.
set -- $VERSION
MAJOR=$1
MINOR=$2
unset IFS
MINOR_TAG="${MAJOR}.${MINOR}"

ACTION="--push"
if [ "$MODE" = "load" ]; then
  ACTION="--load"
  PLATFORMS=${PLATFORMS%%,*}
fi

CMD="docker buildx build --platform $PLATFORMS -t $REPO:$VERSION -t $REPO:$MINOR_TAG -t $REPO:latest $ACTION $ROOT_DIR"

if [ "$MODE" = "print" ]; then
  printf '%s\n' "$CMD"
else
  exec sh -c "$CMD"
fi
