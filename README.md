# KOsync

KOsync is a progress sync server for KOReader written in Go.

## Why?

The [official KOReader progress sync server](https://github.com/koreader/koreader-sync-server) is written in Lua using OpenResty.  
For deployment it needs Nginx with OpenResty as well as Redis as database.

KOsync wants to be simpler by not having any dependencies besides the OS itself.  
(If you need TLS, a reverse proxy is also required, I recommend [Caddy](https://caddyserver.com))

In addition to requiring Nginx, OpenResty and Redis, the official server is not very maintained.  
The last feature adding commits was around 2016.

While KOsync does not yet have additional features compared to the official server,  
there are plans to add some. A web interface for viewing and managing would be nice, right?

## KOsync vs [KOReader Sync Server](https://github.com/koreader/koreader-sync-server)

You may choose KOsync over [KORSS](https://github.com/koreader/koreader-sync-server) due to the following differences:

- Currently maintained
- Open-minded to implement new features, be it a Web Interface or something else
- Written in Go and deploys as a single executable
- Single JSON file as database plus configuration instead of Redis

Additional differences, that should be known:

- KOsync is licensed under the EUPL-1.2 (or later) compared to KORSS, which is AGPL-3.0 or later
- Simple deployment via Docker
- Requires a Reverse Proxy for TLS

### Simplicity

**Simple Code**  
KOsync is written in Go and uses the standard library as much as possible.

Compilation only requires the Go Toolchain with this command `go build -tags netgo -o kosync.exe cmd/kosync/kosync.go`.  
The command compiles KOsync to a single static executable named `kosync.exe`.

Alternatively, a Docker image can be build with `docker buildx build -f build/package/Dockerfile -t docker.obth.eu/atjontv/kosync:custom .`.  
Every tagged version also has a pre-build image at `docker.obth.eu/atjontv/kosync:latest` (you can replace `latest` with the version too).

**Simple Datastore**  
KOsync stores all data, both configuration and user data, in a single JSON file.

The Schema of the file is shown and explained in the next section.

Users can, after entering the custom URL, use the KOReader registration to signup.  
After that, they push and pull progress states.

Documents are uploaded by KOReader during progress push.  
The push must be triggered by hand or configured to be done automatically when switching pages.  
Consult the KOReader documentation for the configuration options.

### Database File

See [docs/database.md]()

### Backup Files

See [docs/backups.md]()

### API Specification

See [docs/api.md]()

## License

KOsync is licensed under the [European Union Public License v1.2 or later](/LICENSE.txt)
