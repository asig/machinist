VERSION ?= dev

.PHONY: build test check static cross

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/machinist ./cmd/machinist

test:
	go test ./...

check:
	@if [ -n "$$(gofmt -l .)" ]; then \
		gofmt -l .; \
		exit 1; \
	fi
	go vet ./...
	./scripts/check-headers.sh

static: build
	@output="$$(ldd bin/machinist 2>&1)"; \
	printf '%s\n' "$$output"; \
	printf '%s\n' "$$output" | grep -Fq "not a dynamic executable"

cross:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o bin/machinist-linux-arm64 ./cmd/machinist
