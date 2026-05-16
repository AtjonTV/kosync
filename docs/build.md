## Build

There are three ways to build KOsync:
- Static Executable (with or without WebUI)
- Dynamic Executable (without WebUI)
- Docker Image (with WebUI)

I recommend using the Docker Image, but if you want to run KOsync on a system without Docker,  
you should use a Static Executable as it does not depend on other libraries.

For deployment, I recommend using Docker Compose, but you can choose whatever method you like.

In any case a Reverse Proxy like Caddy is required for TLS and generally recommended.

### Static Executable (recommended when docker is not an option)
Compilation requires the Go Toolchain and Bun.

Run this command to build:
```bash
go run build.go
```

To build without the WebUI, and thus without Bun, run:
```bash
go run build.go -web=false
```

These commands compile KOsync to a single static executable named `kosync.exe`.

When build with the WebUI, the WebUI needs to be enabled at runtime by passing `--webui` via CLI or by setting the `ENABLE_WEBUI` environment variable.

See the output of `go run build.go -help` for additional options.

### Dynamic Executable (`go install`)
Compilation requires the Go Toolchain.

Run `go install -tags netgo git.obth.eu/atjontv/kosync@latest` to install the latest version (you can also replace `@latest` with a version tag like `@v26.05.0`).
The binary `kosync` will be placed in `$GOPATH/bin`, which is usually `$HOME/go/bin`.

This method will **NOT** contain the WebUI because `go generate` will not be run.

### Docker Container (Recommended)

Run this command in the project root directory: `docker buildx build -f deployment/Dockerfile -t docker.obth.eu/atjontv/kosync:custom .`.

This will build a docker image with the WebUI included. To use the WebUI you either have to override the entrypoint to add `--webui` or set `the `ENABLE_WEBUI` environment variable.

Each tagged release will have a pre-build image at `docker.obth.eu/atjontv/kosync:latest` (you can replace `latest` with a version tag like `26.05.0` so you know what version you pulled).

## Deploy

### Docker Compose (Recommended)

The easiest way to run KOsync in a reproducible environment is via Docker Compose.

A pre-made Compose file is located at `deployment/compose.yml`.

The Compose file includes commented-out examples how to build from Source or to map the container ports to your local machine.

It is recommended to use Caddy as a reverse proxy in front of the container.  
An example Caddyfile is located at `deployment/Caddyfile`.

### Executable

If you choose to run KOsync from one of the executable installation methods, you can simply execute the binary.

It is recommended to use Caddy as a reverse proxy in front of KOsync.  
An example Caddyfile is located at `deployment/Caddyfile`.
