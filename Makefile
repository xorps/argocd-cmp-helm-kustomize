GO_VERSION   := $(shell awk '/^golang /{print $$2}' .tool-versions)
IMAGE        ?= ghcr.io/xorps/argocd-cmp-helm-kustomize
TAG          ?= dev

.PHONY: all build test fmt fmt-check vet lint tidy-check check image

all: check build

build:
	go build -trimpath -ldflags "-s -w" -o bin/cmp-helm-kustomize ./cmd/cmp-helm-kustomize

test:
	go test -race ./...

fmt:
	gofmt -w .

fmt-check:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	go vet ./...

lint:
	golangci-lint run ./...

tidy-check:
	go mod tidy -diff

check: fmt-check vet lint tidy-check test

image:
	docker build --build-arg GO_VERSION=$(GO_VERSION) -t $(IMAGE):$(TAG) .
