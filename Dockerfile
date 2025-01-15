FROM --platform=${BUILDPLATFORM} golang:1.23.3 AS builder

# Install git.
# Git is required for fetching the dependencies.
#RUN apk update && apk add --no-cache git tzdata
WORKDIR $GOPATH/src/wimaha/teslaBleHttpProxy/
COPY . .
# Fetch dependencies.
# Using go get.
#RUN go get -d -v

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG GOARM=${TARGETVARIANT#v}

#WORKDIR /app/
#ADD . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${GOARM} make build-docker
#RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${GOARM} go build -ldflags="-w -s" -o /go/bin/teslaBleHttpProxy main.go
RUN mkdir -p /go/bin/key

FROM alpine:3.21

# Timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
ENV TZ=Europe/Berlin
#WORKDIR /app/
COPY --from=builder /go/bin/teslaBleHttpProxy /teslaBleHttpProxy
COPY --from=builder /go/bin/key /key
COPY healthcheck.sh /healthcheck.sh
RUN chmod +x /healthcheck.sh
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3  CMD /healthcheck.sh

ENTRYPOINT ["/teslaBleHttpProxy"]
