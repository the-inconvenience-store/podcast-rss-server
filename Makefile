GOCACHE ?= $(CURDIR)/.cache/go-build

.PHONY: test test-integration lint run compose-up compose-down

test:
	GOCACHE=$(GOCACHE) go test ./...

test-integration:
	GOCACHE=$(GOCACHE) go test -tags=integration ./...

lint:
	GOCACHE=$(GOCACHE) go vet ./...
	GOCACHE=$(GOCACHE) go run honnef.co/go/tools/cmd/staticcheck@latest ./...

run:
	GOCACHE=$(GOCACHE) go run ./cmd/podcast-rss

compose-up:
	docker compose up --build

compose-down:
	docker compose down -v
