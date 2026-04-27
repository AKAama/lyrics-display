APP_NAME := lyrics-display
VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo local)
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

.PHONY: build run version clean dist

build:
	go build -ldflags "$(LDFLAGS)" -o $(APP_NAME) .

run:
	go run -ldflags "$(LDFLAGS)" .

version:
	go run -ldflags "$(LDFLAGS)" . --version

clean:
	rm -f $(APP_NAME)

dist:
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-darwin-arm64 .
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-darwin-amd64 .
