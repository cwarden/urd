.PHONY: build clean test fmt lint install

BINARY_NAME=urd
GO_FILES=$(shell find . -type f -name '*.go')

build: $(BINARY_NAME)

$(BINARY_NAME): $(GO_FILES)
	go build -o $(BINARY_NAME) ./cmd/urd

clean:
	go clean
	rm -f $(BINARY_NAME)

test:
	go test ./...

fmt:
	gofmt -w .

lint:
	go vet ./...

install: build
	go install ./cmd/urd

dev: fmt test build

.DEFAULT_GOAL := build
