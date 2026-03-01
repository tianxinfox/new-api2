FROM oven/bun:latest AS builder

WORKDIR /build
COPY web/package.json .
COPY web/bun.lock .
RUN bun install
COPY ./web .
COPY ./VERSION .
RUN DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat VERSION) bun run build

FROM golang:alpine AS builder2
ENV GO111MODULE=on CGO_ENABLED=0

ARG TARGETOS
ARG TARGETARCH
ENV GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}
ENV GOEXPERIMENT=greenteagc

WORKDIR /build

ADD go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=builder /build/dist ./web/dist
RUN go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'" -o new-api

FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive

RUN set -eux; \
    echo 'Acquire::Retries "5"; Acquire::http::Timeout "20"; Acquire::https::Timeout "20"; Acquire::ForceIPv4 "true";' > /etc/apt/apt.conf.d/99retries; \
    for i in 1 2 3; do \
        apt-get update --fix-missing && break; \
        echo "apt-get update failed, retry ${i}/3"; \
        sleep $((i * 5)); \
        if [ "$i" -eq 3 ]; then exit 1; fi; \
    done; \
    for i in 1 2 3; do \
        apt-get install -y --no-install-recommends --fix-missing ca-certificates tzdata && break; \
        echo "apt-get install failed, retry ${i}/3"; \
        sleep $((i * 5)); \
        if [ "$i" -eq 3 ]; then exit 1; fi; \
        apt-get update --fix-missing; \
    done; \
    update-ca-certificates; \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder2 /build/new-api /
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/new-api"]
