.PHONY: build test test-race vet check cross-build validate-plugins

build:
	mkdir -p bin
	go build -o bin/memory ./cmd/memory

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

cross-build:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /tmp/memory-windows-amd64.exe ./cmd/memory
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o /tmp/memory-windows-arm64.exe ./cmd/memory
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o /tmp/memory-darwin-amd64 ./cmd/memory
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /tmp/memory-darwin-arm64 ./cmd/memory
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/memory-linux-amd64 ./cmd/memory
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /tmp/memory-linux-arm64 ./cmd/memory

check: vet test test-race cross-build
