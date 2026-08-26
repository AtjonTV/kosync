## Multi-stage build: 1. compile the webui and a static binary, 2. run it against a /pb_data volume

# 1) Build stage
FROM golang:1.26.6-alpine3.23 AS builder

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
RUN apk add --cache=no ca-certificates git build-base nodejs npm && npm install -g bun@1.3.13

# Cache the Go modules first
COPY server/go.mod server/go.sum ./server/
RUN cd server && go mod download

# Cache the WebUI dependencies next
COPY webui/package.json webui/bun.lock ./webui/
RUN cd webui && bun install --frozen-lockfile

# Copy the rest of the source
COPY . .

# Build the WebUI into the Go embed directory
RUN cd server && go generate ./internal/webui

# Build a static binary named 'kosync'
ENV CGO_ENABLED=0
RUN cd server && go build -tags netgo -ldflags='-s -w -extldflags "-static"' -o /out/kosync . && strip /out/kosync


# 2) Runtime stage
FROM alpine:3.23.3

# Install CA certs for outbound HTTPS (SMTP, S3 backups), if needed
RUN apk add --cache=no ca-certificates && update-ca-certificates

COPY --from=builder /out/kosync /app/kosync

EXPOSE 8080

USER 1000:1000

# PocketBase keeps the database, the uploads and its backups in here.
VOLUME /pb_data
WORKDIR /pb_data

ENTRYPOINT ["/app/kosync"]
CMD ["serve", "--http=0.0.0.0:8080", "--dir=/pb_data"]
