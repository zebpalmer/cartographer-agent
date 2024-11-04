.PHONY: help all style test release format lint check-branch check-clean patch minor major build pull-tags mod-tidy dev-setup

## Project Variables
PROJECT  := cartographer-agent
BINARY   := cartographer-agent
REPO     := github.com/zebpalmer/cartographer-go-agent
HASH     := $(shell git rev-parse --short HEAD)
TIMESTAMP := $(shell date '+%Y-%m-%dT%H:%M:%S')

## Use "dev" for local builds and get the latest tag for releases
LOCAL_VERSION := dev
VERSION  := $(shell git describe --tags --abbrev=0 --match="v[0-9]*.[0-9]*.[0-9]*" 2> /dev/null || echo "development")

## Build and Release Variables
BASE_BUILD_FOLDER := build/_output
VERSION_FOLDER    := ${PROJECT}-${VERSION}
BUILD_FOLDER      := ${BASE_BUILD_FOLDER}/${VERSION_FOLDER}
RELEASE_FOLDER    := release/${PROJECT}-${VERSION}

LDFLAGS_DEV     := "-X 'main.Version=${LOCAL_VERSION}' -X 'main.CommitHash=${HASH}' -X 'main.BuildTimestamp=${TIMESTAMP}'"
LDFLAGS_RELEASE := -ldflags "-X 'main.Version=${VERSION}' -X 'main.CommitHash=${HASH}' -X 'main.BuildTimestamp=${TIMESTAMP}'"



## General Targets
all: pull-tags style test build  ## Format, lint, test, and build the project

## Tidy Go modules
mod-tidy: ## Clean up go.mod and go.sum
	go mod tidy

## Pull all tags to ensure we're up-to-date
pull-tags:
	git fetch --tags

## Format and lint code
style: format lint  ## Format and lint code

## Building
build: ## Build the Go application for multiple platforms
	echo "Compiling Cartographer for all supported OS and Platforms"
	# cleanup old builds
	rm -f build/_output/cartographer-agent_*
	# Linux
	env GOOS=linux GOARCH=amd64 go build -ldflags=${LDFLAGS_DEV} -o build/_output/cartographer-agent_linux-amd64
	env GOOS=linux GOARCH=arm GOARM=6 go build -ldflags=${LDFLAGS_DEV} -o build/_output/cartographer-agent_linux-arm6
	env GOOS=linux GOARCH=arm GOARM=7 go build -ldflags=${LDFLAGS_DEV} -o build/_output/cartographer-agent_linux-arm7
	# Darwin
	env GOOS=darwin GOARCH=arm64 go build -ldflags=${LDFLAGS_DEV} -o build/_output/cartographer-agent_darwin-arm64
	env GOOS=darwin GOARCH=amd64 go build -ldflags=${LDFLAGS_DEV} -o build/_output/cartographer-agent_darwin-amd64

## Testing
test: ## Run tests
	go test ./... -v

## Code Formatting
format: ## Format code using gofmt and goimports
	go fmt ./...
	goimports -w .

## Code Linting
lint: ## Lint code using golint and go vet
	go vet ./...
	golint ./...

## Release Operations
release: check-type check-branch pre-release $(TYPE) post-release  ## Perform a release
	@echo "Release complete"

######### Helpers (not meant to be called directly) #########
# release checking
pre-release: check-branch mod-tidy style check-clean test build
post-release: push

# Check if the current Git branch is 'main'
check-branch:
	@if [ "$(shell git rev-parse --abbrev-ref HEAD)" != "main" ]; then \
		echo "You are not on the 'main' branch. Aborting."; \
		exit 1; \
	fi

check-type:
	@if [ -z "$(TYPE)" ]; then \
		echo "ERROR: Specify 'patch', 'minor', 'major' by running 'make release TYPE=<type>'"; \
		exit 1; \
	fi
	@if [ "$(TYPE)" != "patch" -a "$(TYPE)" != "minor" -a "$(TYPE)" != "major" ]; then \
		echo "ERROR: Invalid TYPE set. Specify 'patch', 'minor', 'major' by running 'make release TYPE=<type>'"; \
		exit 1; \
	fi

## Git Operations
push:
	git push && git push --tags

## Git Status Check
check-clean:
	@if [ -n "$(shell git status --porcelain)" ]; then \
		echo "Your Git working directory is not clean. Commit or stash your changes before proceeding."; \
		exit 1; \
	fi

## Version Bumping
patch: pull-tags ## Bump patch version
	@$(eval CURRENT_VERSION=$(shell git describe --tags --abbrev=0 --match="v[0-9]*.[0-9]*.[0-9]*" 2> /dev/null || echo "0.0.0"))
	@$(eval NEW_VERSION=$(shell echo $(CURRENT_VERSION) | awk -F. '{$$3+=1; print $$1"."$$2"."$$3}'))
	git tag -a $(NEW_VERSION) -m "Patch release $(NEW_VERSION)"
	@echo "Patch version bumped to $(NEW_VERSION)."

minor: pull-tags ## Bump minor version
	@$(eval CURRENT_VERSION=$(shell git describe --tags --abbrev=0 --match="v[0-9]*.[0-9]*.[0-9]*" 2> /dev/null || echo "0.0.0"))
	@$(eval NEW_VERSION=$(shell echo $(CURRENT_VERSION) | awk -F. '{$$2+=1; $$3=0; print $$1"."$$2"."$$3}'))
	git tag -a $(NEW_VERSION) -m "Minor release $(NEW_VERSION)"
	@echo "Minor version bumped to $(NEW_VERSION)."

major: pull-tags ## Bump major version
	@$(eval CURRENT_VERSION=$(shell git describe --tags --abbrev=0 --match="v[0-9]*.[0-9]*.[0-9]*" 2> /dev/null || echo "0.0.0"))
	@$(eval NEW_VERSION=$(shell echo $(CURRENT_VERSION) | awk -F. '{$$1+=1; $$2=0; $$3=0; print $$1"."$$2"."$$3}'))
	git tag -a $(NEW_VERSION) -m "Major release $(NEW_VERSION)"
	@echo "Major version bumped to $(NEW_VERSION)."

## Development Environment Setup
dev-setup: ## Install development tools (golint, goimports)
	go install golang.org/x/lint/golint@latest
	go install golang.org/x/tools/cmd/goimports@latest
