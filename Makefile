.PHONY: build test

BIN ?= yamdc

build:
	go build -o $(BIN) ./cmd/yamdc

test:
	go test ./...
