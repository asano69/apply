BINARY := apply

.PHONY: build install uninstall fmt vet test clean

build:
	go build -o $(BINARY) ./cmd/apply

install:
	go install .

uninstall:
	rm -f $(shell go env GOPATH)/bin/$(BINARY)

fmt:
	gofmt -l .

vet:
	go vet ./...

test: fmt vet
	go build -o /dev/null ./cmd/apply
	go test ./...

clean:
	rm -f $(BINARY)
