# syntax=docker/dockerfile:1
# SPDX-License-Identifier: Apache-2.0 OR MIT
ARG GO_VERSION=1.27.1

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
RUN --mount=type=bind,source=go.mod,target=go.mod \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=cmd,target=cmd \
    --mount=type=bind,source=internal,target=internal \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w" -o /out/cmp-helm-kustomize ./cmd/cmp-helm-kustomize

# argocd-cmp-server is mounted from the repo-server at runtime.
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/cmp-helm-kustomize /usr/local/bin/cmp-helm-kustomize
COPY plugin.yaml /home/argocd/cmp-server/config/plugin.yaml
# Argo CD runs plugin sidecars as uid 999.
USER 999:999
