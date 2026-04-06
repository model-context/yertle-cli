PROJECT := yertle
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build run clean test release release-dry-run

build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o $(PROJECT) .

run: build
	./$(PROJECT)

clean:
	rm -f $(PROJECT)

test:
	go test ./...

release:
	@if [ -z "$$GITHUB_TOKEN" ]; then echo "GITHUB_TOKEN is not set"; exit 1; fi
	goreleaser release --clean

release-dry-run:
	goreleaser release --snapshot --clean
