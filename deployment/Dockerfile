## Multi-stage build: compile a single static Go binary, run it, and allow mounting database.json

# 1) Build stage
FROM golang:1.25.6-alpine3.23 AS builder

WORKDIR /src

# Install build deps
RUN apk add --cache=no ca-certificates git build-base nodejs npm && npm install -g bun@1.3.5

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

WORKDIR /app

# Install CA certs for HTTPS outbound, if needed
RUN apk add --no-cache ca-certificates && update-ca-certificates

# Copy the binary
COPY --from=builder /out/kosync /app/kosync

# The application listens on 8080 by default
EXPOSE 8080

VOLUME /data

ENTRYPOINT ["/app/kosync"]
