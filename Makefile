# InfraSync Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOFORMAT=$(GOCMD) fmt
GOLINT=golangci-lint
BINARY_NAME=infrasync
CMD_DIR=./cmd
MAIN_GO=./main.go

# Build targets
.PHONY: all build clean run test lint fmt help

all: clean fmt lint test build

build:
	@echo "Building..."
	$(GOBUILD) -o $(BINARY_NAME) $(MAIN_GO)

install:
	$(GOCMD) install $(MAIN_GO)

clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

run:
	@echo "Running..."
	$(GORUN) $(MAIN_GO)

test:
	@echo "Testing..."
	$(GOTEST) -v ./...

lint:
	@echo "Linting..."
	$(GOLINT) run

fmt:
	@echo "Formatting..."
	$(GOFORMAT) -s -w .

# Specific commands
.PHONY: init import sync

init:
	@echo "Running init command..."
	./$(BINARY_NAME) init $(filter-out $@,$(MAKECMDGOALS))

import:
	@echo "Running import command..."
	./$(BINARY_NAME) import $(filter-out $@,$(MAKECMDGOALS))

sync:
	@echo "Running sync command..."
	./$(BINARY_NAME) sync $(filter-out $@,$(MAKECMDGOALS))

# Special target to allow passing arguments to specific commands
%:
	@:

help:
	@echo "InfraSync Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build        Build the binary"
	@echo "  clean        Clean build files"
	@echo "  run          Run directly with go run"
	@echo "  test         Run tests"
	@echo "  lint         Run linter"
	@echo "  fmt          Format code"
	@echo "  init         Run init command"
	@echo "  import       Run import command"
	@echo "  sync         Run sync command"
	@echo "  help         Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  make import --project=my-project --services=pubsub"
	@echo "  make sync --project=my-project --state-bucket=my-bucket"
	@echo ""
