############################
# STEP 1 build executable binary
############################
#FROM golang:alpine AS builder
FROM --platform=$BUILDPLATFORM golang:alpine AS builder
# Install git.
# Git is required for fetching the dependencies.
RUN apk update && apk add --no-cache git tzdata
WORKDIR $GOPATH/src/wimaha/teslaBleHttpProxy/
COPY . .
# Fetch dependencies.
# Using go get.
RUN go get -d -v
# Build the binary.
#RUN go build -o /go/bin/teslaBleHttpProxy
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-w -s" -o /go/bin/teslaBleHttpProxy
#RUN GOOS=linux go build -ldflags="-w -s" -o /go/bin/teslaBleHttpProxy
#RUN sudo setcap 'cap_net_admin=eip' "/go/bin/teslaBleHttpProxy"
RUN mkdir -p /go/bin/key
############################
# STEP 2 build a small image
############################
FROM scratch
# Timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
ENV TZ=Europe/Berlin
# Copy our static executable.
COPY --from=builder /go/bin/teslaBleHttpProxy /teslaBleHttpProxy
COPY --from=builder /go/bin/key /key
EXPOSE 8080
# Run the teslaBleHttpProxy binary.
ENTRYPOINT ["/teslaBleHttpProxy"]