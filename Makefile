GOCACHE ?= $(CURDIR)/.cache/go-build
VERSION ?= $(shell tr -d '[:space:]' < VERSION)

.PHONY: test test-integration test-release-scripts lint run compose-up compose-down docker-plan docker-push docker-load version-patch version-minor version-major

test:
	GOCACHE=$(GOCACHE) go test ./...

test-integration:
	GOCACHE=$(GOCACHE) go test -tags=integration ./...

test-release-scripts:
	scripts/test-release-scripts.sh

lint:
	GOCACHE=$(GOCACHE) go vet ./...
	GOCACHE=$(GOCACHE) go run honnef.co/go/tools/cmd/staticcheck@latest ./...

run:
	GOCACHE=$(GOCACHE) go run ./cmd/podcast-rss

compose-up:
	docker compose up --build

compose-down:
	docker compose down -v

docker-plan:
	scripts/docker-publish.sh --print $(VERSION)

docker-push:
	scripts/docker-publish.sh --push $(VERSION)

docker-load:
	scripts/docker-publish.sh --load $(VERSION)

version-patch:
	scripts/bump-version.sh patch

version-minor:
	scripts/bump-version.sh minor

version-major:
	scripts/bump-version.sh major
