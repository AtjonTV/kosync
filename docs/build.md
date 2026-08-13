# Building

KOsync needs two tools: **Go** for the server and **Bun** for the web interface. Nothing else.

## The whole thing

```bash
cd server
go run build.go
```

This builds the web interface with Bun, embeds it into the Go binary and writes `./kosync`.

Options:

| Flag | Default | Meaning |
| --- | --- | --- |
| `-web` | `true` | build the web interface first; `-web=false` reuses what is already embedded |
| `-out` | `./kosync` | name of the produced executable |
| `-go` | `go` | path to the Go toolchain |
| `-run` | `false` | start the server after building |

## Only the server

```bash
cd server
go build -tags netgo -o kosync .
```

The web interface is embedded from `internal/webui/public`. When that directory holds nothing but its
`.keep` placeholder, the binary runs fine but serves no interface, and says so in the log.

## Only the web interface

```bash
cd server
go generate ./internal/webui
```

This runs `bun install` and `bun build-only` and writes the result into `internal/webui/public`.

## Developing

Run the server and the Vite dev server side by side:

```bash
cd server && go run . serve            # http://127.0.0.1:8090
cd webui  && bun run dev               # http://127.0.0.1:5173, proxies /api to the server
```

## Tests

```bash
cd server && go test ./...
cd webui  && bun run test
```

The Go tests create a throwaway PocketBase instance per test and run the real migrations against it,
so they cover the schema and its access rules as well as the handlers.

## Docker

```bash
docker build -f deployment/Dockerfile -t kosync .
docker run -v ./data:/pb_data -e ENABLE_WEBUI=true -p 8080:8080 kosync
```
