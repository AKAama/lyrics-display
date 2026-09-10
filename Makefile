APP_NAME := lyrics-display
VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo local)
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

.PHONY: build run version test clean dist app dmg

build:
	go build -ldflags "$(LDFLAGS)" -o $(APP_NAME) .

run:
	go run -ldflags "$(LDFLAGS)" .

version:
	go run -ldflags "$(LDFLAGS)" . --version

test:
	go test ./...

clean:
	rm -f $(APP_NAME)
	rm -rf dist

dist:
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-darwin-arm64 .
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-darwin-amd64 .

app:
	VERSION=$(VERSION) bash packaging/build-app.sh

dmg:
	VERSION=$(VERSION) bash packaging/build-app.sh --dmg
