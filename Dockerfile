# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG APPVERSION="development"

RUN echo "Building on $BUILDPLATFORM for $TARGETPLATFORM"

WORKDIR /build
COPY . .

# Set the target platform architecture
RUN case "$TARGETPLATFORM" in \
    "linux/amd64") GOARCH=amd64 ;; \
    "linux/arm64") GOARCH=arm64 ;; \
    "linux/arm/v7") GOARCH=arm GOARM=7 ;; \
    "linux/arm/v6") GOARCH=arm GOARM=6 ;; \
    *) echo "Unsupported platform: $TARGETPLATFORM" && exit 1 ;; \
    esac && \
    GOOS=linux CGO_ENABLED=0 GOARCH=$GOARCH GOARM=$GOARM go build -o teslablehttpproxy -ldflags "-s -w -X main.Version=$APPVERSION" -trimpath

FROM alpine:latest

COPY --from=builder /build/teslablehttpproxy /usr/local/bin/

ENTRYPOINT ["teslablehttpproxy"]

LABEL org.opencontainers.image.description="Tesla BLE HTTP Proxy"
LABEL org.opencontainers.image.source="https://github.com/Lenart12/TeslaBleHttpProxy"
