.PHONY: all build test clean run fmt vet

# Default target
all: build test

# Build the project
build:
	go build -o bin/basalt ./cmd

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Run the project
run:
	go run ./cmd

# Run the built binary
run-bin: build
	./bin/basalt

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Install dependencies
deps:
	go mod tidy

# Create bin directory
bin:
	mkdir -p bin
