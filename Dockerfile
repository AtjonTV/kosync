## Multi-stage build: compile a single static Go binary, run it, and allow mounting database.json

# 1) Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Install build deps (just in case) and CA certs
RUN apk add --no-cache ca-certificates git build-base

# Cache go modules first
COPY go.mod ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Build a static binary named 'main'
ENV CGO_ENABLED=0
RUN go build -tags netgo -ldflags='-s -w -extldflags "-static"' -o /out/main ./main.go && strip /out/main


# 2) Runtime stage
FROM alpine:3.23

WORKDIR /app

# Install CA certs for HTTPS outbound, if needed
RUN apk add --no-cache ca-certificates && update-ca-certificates

# Copy the binary
COPY --from=builder /out/main /app/main

# The application listens on 8080 by default
EXPOSE 8080

# database.json is expected in the same directory as the binary (/app).
# Mount it at runtime if you want to persist or provide an existing DB:
#   docker run --rm -p 8080:8080 \
#     -v $(pwd)/database.json:/app/database.json \
#     ghcr.io/your-org/kosync:latest

ENTRYPOINT ["/app/main"]
