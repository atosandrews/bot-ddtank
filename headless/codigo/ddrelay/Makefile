.PHONY: build test race vet format
build:
	mkdir -p bin
	CGO_ENABLED=0 go build -o bin/ddrelay ./cmd/ddrelay
test:
	go test ./...
race:
	go test -race ./...
vet:
	go vet ./...

format:
	gofmt -w cmd internal
