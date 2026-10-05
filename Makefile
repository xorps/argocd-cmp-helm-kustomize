# SPDX-License-Identifier: Apache-2.0 OR MIT
GO_VERSION   := $(shell awk '/^golang /{print $$2}' .tool-versions)
IMAGE        ?= ghcr.io/xorps/argocd-cmp-helm-kustomize
TAG          ?= dev

.PHONY: all build test fmt fmt-check vet lint tidy-check license-check check image

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

SPDX     := SPDX-License-Identifier: Apache-2.0 OR MIT
LICENSED  = $(shell git ls-files '*.go' '*.sh' '*.yaml' '*.yml' Dockerfile Makefile ':(exclude)examples/')

license-check:
	@missing=0; for f in $(LICENSED); do \
		head -n 3 "$$f" | grep -qF '$(SPDX)' || { echo "missing '$(SPDX)': $$f"; missing=1; }; \
	done; exit $$missing

check: fmt-check vet lint tidy-check license-check test

image:
	docker build --build-arg GO_VERSION=$(GO_VERSION) -t $(IMAGE):$(TAG) .
