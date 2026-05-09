## Multi-stage build: 1. compile webui and static binary, 2. run the binary with possible /data volume

# 1) Build stage
FROM golang:1.26.0-alpine3.23 AS builder

RUN tee > /etc/apk/repositories <<EOF
## Official Repos
#https://dl-cdn.alpinelinux.org/alpine/v3.23/main
#https://dl-cdn.alpinelinux.org/alpine/v3.23/community

## obth.eu LSUS
https://lsus.obth.eu/alpine/v3.23/main
https://lsus.obth.eu/alpine/v3.23/community
EOF

WORKDIR /src

# Install build deps
RUN apk add --cache=no ca-certificates git build-base nodejs npm && npm install -g bun@1.3.9

# Cache go modules first
COPY ../go.mod ./
RUN go mod download

# Copy the rest of the source
COPY .. .

# Build webui
RUN go generate kosync.go

# Build a static binary named 'kosync'
ENV CGO_ENABLED=0
RUN go build -tags netgo -ldflags='-s -w -extldflags "-static"' -o /out/kosync kosync.go && strip /out/kosync


# 2) Runtime stage
FROM alpine:3.23.3

# Install CA certs for HTTPS outbound, if needed
RUN apk add --cache=no ca-certificates && update-ca-certificates

# Copy the binary
COPY --from=builder /out/kosync /app/kosync

# The application listens on 8080 by default
EXPOSE 8080

USER 1000:1000

VOLUME /data
WORKDIR /data

ENTRYPOINT ["/app/kosync"]
