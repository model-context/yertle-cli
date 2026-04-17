PROJECT := yertle
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# Optional .env file (gitignored) provides GITHUB_TOKEN and any other secrets.
# Lines like `GITHUB_TOKEN=ghp_...` are auto-exported into recipe shells.
-include .env
export

# Default bump component for `make release`. Override: make release BUMP=minor
BUMP ?= patch

.PHONY: help build run clean test release release-dry-run

help: ## Show this help message
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[32mmake %-16s\033[0m %s\n", $$1, $$2}'

build: ## Build the yertle binary
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o $(PROJECT) .

run: build ## Build and run the yertle binary
	./$(PROJECT)

clean: ## Remove the built binary
	rm -f $(PROJECT)

test: ## Run all Go tests
	go test ./...

release-dry-run: ## Test the release pipeline locally (no publish)
	goreleaser release --snapshot --clean

# Tag + publish a new release. Defaults: patch-bumps the latest tag, requires
# a clean working tree on main, requires GITHUB_TOKEN (from .env or shell),
# prompts for confirmation before tagging.
#
# Override examples:
#   make release                   # v0.1.0 -> v0.1.1
#   make release BUMP=minor        # v0.1.0 -> v0.2.0
#   make release BUMP=major        # v0.1.0 -> v1.0.0
#   make release TAG=v0.5.0        # explicit
release: ## Tag and publish a new release (BUMP=patch|minor|major)
	@set -e; \
	if [ -z "$$GITHUB_TOKEN" ]; then \
		echo "ERROR: GITHUB_TOKEN is not set."; \
		echo "       Put it in ./.env (gitignored) or export it in your shell."; \
		exit 1; \
	fi; \
	if [ -n "$$(git status --porcelain)" ]; then \
		echo "ERROR: working tree is dirty. Commit or stash first."; \
		git status --short; \
		exit 1; \
	fi; \
	branch="$$(git rev-parse --abbrev-ref HEAD)"; \
	if [ "$$branch" != "main" ]; then \
		echo "ERROR: not on main (currently on $$branch). Releases must come from main."; \
		exit 1; \
	fi; \
	git fetch --tags --quiet; \
	latest="$$(git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)"; \
	if [ -n "$(TAG)" ]; then \
		next="$(TAG)"; \
	else \
		v="$${latest#v}"; \
		maj="$${v%%.*}"; rest="$${v#*.}"; \
		min="$${rest%%.*}"; pat="$${rest#*.}"; \
		case "$(BUMP)" in \
			major) maj=$$((maj+1)); min=0; pat=0 ;; \
			minor) min=$$((min+1)); pat=0 ;; \
			patch) pat=$$((pat+1)) ;; \
			*) echo "ERROR: BUMP must be major|minor|patch (got '$(BUMP)')"; exit 1 ;; \
		esac; \
		next="v$$maj.$$min.$$pat"; \
	fi; \
	if git rev-parse "$$next" >/dev/null 2>&1; then \
		echo "ERROR: tag $$next already exists."; \
		exit 1; \
	fi; \
	echo ""; \
	echo "  Current tag : $$latest"; \
	echo "  New tag     : $$next"; \
	echo "  Branch      : $$branch ($$(git rev-parse --short HEAD))"; \
	echo ""; \
	printf "Proceed with release? [y/N] "; \
	read ans; \
	if [ "$$ans" != "y" ] && [ "$$ans" != "Y" ]; then \
		echo "Aborted."; exit 1; \
	fi; \
	git tag "$$next"; \
	git push origin "$$next"; \
	goreleaser release --clean
