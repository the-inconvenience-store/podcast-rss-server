# podcast-rss-server

A small Go HTTP service for publishing Apple Podcasts-compatible RSS feeds. Metadata lives in SQLite, audio and artwork live in an S3-compatible bucket, and feeds are generated from current data on every request.

## Features

- Multiple podcast shows with episodes.
- Public RSS feeds at `/` and `/feeds/{showID}.xml`.
- Public media proxy at `/media/{showID}/{episodeID}/{filename}` with `Range` support via `http.ServeContent`.
- API-key protected `/api/...` routes for show, episode, audio, and artwork management.
- JPEG/PNG artwork validation for Apple bounds: square, 1400x1400 through 3000x3000.
- SQLite through pure-Go `modernc.org/sqlite`.
- S3-compatible storage through AWS SDK for Go v2 with custom endpoint and path-style addressing for Garage.

## Configuration

Copy `.env.example` to `.env` for local compose usage.

Required environment variables:

- `API_KEYS`: comma-separated API keys.
- `S3_ENDPOINT`, `S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`
- `PUBLIC_BASE_URL`: base URL used inside feeds for media and feed self links.
- `DATABASE_PATH`
- `LISTEN_ADDR`
- `DEFAULT_SHOW_ID`: optional when more than one show exists.

Production must terminate TLS and use an `https://` `PUBLIC_BASE_URL`. Apple requires the feed URL and every asset URL inside it to be publicly reachable over HTTPS.

## Local Garage Stack

Start the app plus Garage:

```sh
cp .env.example .env
make compose-up
```

The compose stack runs:

- `garage` using `dxflrs/garage:v1.0.1`
- `garage-init`, which waits for Garage, assigns the single-node layout, applies layout version 1, creates the `podcasts` bucket, imports deterministic credentials with `garage key import --yes -a <ACCESS_KEY_ID> -s <SECRET>`, and grants bucket read/write/owner access.
- `app` with `S3_ENDPOINT=http://garage:3900`, region `garage`, bucket `podcasts`, and path-style addressing enabled in code.

Stop and remove volumes:

```sh
make compose-down
```

## API Examples

The service generates an OpenAPI 3.1 specification with Huma:

```sh
curl -sS http://localhost:8080/openapi.json
open http://localhost:8080/docs
```

All public feed/media/health endpoints and protected `/api/...` endpoints are included. Protected operations document the bearer API-key requirement.

Create a show:

```sh
curl -sS -X POST http://localhost:8080/api/shows \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "show-1",
    "title": "Quiet Signals",
    "description": "Careful conversations about software.",
    "link": "https://example.com",
    "language": "en-us",
    "author": "Sam Stevens",
    "email": "sam@example.com",
    "category": "Technology",
    "image": "https://example.com/cover.png",
    "explicit": false,
    "type": "episodic"
  }'
```

Create an episode:

```sh
curl -sS -X POST http://localhost:8080/api/shows/show-1/episodes \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "episode-1",
    "title": "Introductions",
    "description": "Meet the show.",
    "publication_date": "2025-01-06T10:00:00Z",
    "duration": "01:01",
    "episode_type": "full"
  }'
```

Upload audio:

```sh
curl -sS -X POST http://localhost:8080/api/shows/show-1/episodes/episode-1/audio \
  -H "Authorization: Bearer $API_KEY" \
  -F "file=@intro.mp3;type=audio/mpeg"
```

Upload artwork:

```sh
curl -sS -X POST http://localhost:8080/api/shows/show-1/image \
  -H "Authorization: Bearer $API_KEY" \
  -F "file=@cover.png;type=image/png"
```

Fetch the public feed without auth:

```sh
curl -sS http://localhost:8080/
curl -sS http://localhost:8080/feeds/show-1.xml
```

Fetch media with a range request:

```sh
curl -v -H "Range: bytes=0-1023" \
  http://localhost:8080/media/show-1/episode-1/intro.mp3
```

## Development

Run tests:

```sh
make test
```

Run integration tests against the compose Garage service:

```sh
make compose-up
RUN_GARAGE_INTEGRATION=1 make test-integration
```

Run lint checks:

```sh
make lint
```

## Docker Hub Releases

The image version is stored in `VERSION`. Bump it with:

```sh
make version-patch
make version-minor
make version-major
```

Set your Docker Hub namespace once per shell:

```sh
export DOCKERHUB_USERNAME=your-dockerhub-user
# or: export DOCKERHUB_REPO=your-dockerhub-user/podcast-rss
```

Preview the buildx command:

```sh
make docker-plan
```

Publish a multi-arch image for Kubernetes:

```sh
docker login
make docker-builder
make docker-push
```

By default this pushes `linux/amd64` and `linux/arm64` tags for `VERSION`, `major.minor`, and `latest`.

Run locally without Docker:

```sh
set -a
. ./.env
set +a
make run
```

Validate a public feed with the RSSBoard validator:

```sh
scripts/validate-feed.sh https://podcasts.example.com/
```

## Data Notes

Episode GUIDs are generated once at creation and stored. Show GUIDs can be supplied explicitly; otherwise the feed generator emits a stable UUIDv5 derived from the feed URL as `<podcast:guid>`.

`duration` accepts integer seconds, `MM:SS`, or `HH:MM:SS`. Feed output uses Apple-compatible `itunes:duration` values.
