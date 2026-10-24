# <!--
# This file is part of Machinist.
#
# Copyright (C) 2026 Andreas Signer <asigner@gmail.com>
#
# Machinist is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# Machinist is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with Machinist.  If not, see <https://www.gnu.org/licenses/>.
# -->

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
