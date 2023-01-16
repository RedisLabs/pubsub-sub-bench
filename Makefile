# Go parameters
GOCMD=GO111MODULE=on go
GOBUILD=$(GOCMD) build
GOBUILDRACE=$(GOCMD) build -race
GOINSTALL=$(GOCMD) install
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
BIN_NAME=pubsub-sub-bench
DISTDIR = ./dist

# Build-time GIT variables
ifeq ($(GIT_SHA),)
GIT_SHA:=$(shell git rev-parse HEAD)
endif

ifeq ($(GIT_DIRTY),)
GIT_DIRTY:=$(shell git diff --no-ext-diff 2> /dev/null | wc -l)
endif

LDFLAGS = "-X 'main.GitSHA1=$(GIT_SHA)' -X 'main.GitDirty=$(GIT_DIRTY)'"

.PHONY: all test coverage build checkfmt fmt
all: test coverage build checkfmt fmt

build:
	$(GOBUILD) \
        -ldflags=$(LDFLAGS) .

build-race:
	$(GOBUILDRACE) \
        -ldflags=$(LDFLAGS) .

checkfmt:
	@echo 'Checking gofmt';\
 	bash -c "diff -u <(echo -n) <(go fmt .)";\
	EXIT_CODE=$$?;\
	if [ "$$EXIT_CODE"  -ne 0 ]; then \
		echo '$@: Go files must be formatted with gofmt'; \
	fi && \
	exit $$EXIT_CODE

lint:
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint
	golangci-lint run

fmt:
	$(GOFMT) .

get:
	$(GOGET) -t -v ./...

test: get
	$(GOFMT) ./...
	$(GOTEST) -race -covermode=atomic ./...

coverage: get test
	$(GOTEST) -race -coverprofile=coverage.txt -covermode=atomic .
