BINARY := semver-bump
PKG := ./...

.PHONY: all build test race vet fmt fmt-check smoke install clean

all: fmt-check vet race build

build:
	go build -o bin/$(BINARY) .

test:
	go test $(PKG)

race:
	go test -race $(PKG)

vet:
	go vet $(PKG)

fmt:
	gofmt -w .

fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi

smoke: build
	bash examples/smoke_test.sh

install:
	go install .

clean:
	rm -rf bin
