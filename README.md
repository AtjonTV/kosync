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

The database file consists of three sections:
- schema
- config
- users

While schema and users is "managed" by the server depending on what is happening,  
the config is where you can configure how KOsync works.

File Example:
```json
{
  "schema": 1,
  "config": {
    "listen_address": ":8080",
    "disable_registration": false,
    "enable_debug_log": false,
    "store_history": false,
    "backup_encoding_type": "msgpack"
  },
  "users": {
    "<username>": {
      "username": "<username>",
      "password": "<password>",
      "documents": {
        "<filehash>": {
          "percentage": 0.10,
          "progress": "/body/DocFragment[9]/body/section/p[110]/text().0",
          "device": "<device>",
          "device_id": "<device_id>",
          "timestamp": 3
        }
      },
      "history": {
        "<filehash>": [
          {
            "percentage": 0.02,
            "progress": "/body/DocFragment[3]/body/section/p[110]/text().0",
            "device": "<device>",
            "device_id": "<device_id>",
            "timestamp": 1
          },
          {
            "percentage": 0.06,
            "progress": "/body/DocFragment[1]/body/section/p[110]/text().0",
            "device": "<device>",
            "device_id": "<device_id>",
            "timestamp": 2
          }
        ]
      }
    }
  }
}
```
**Schema**
* `schema`: Is set by the server and is used for schema alterations/migrations

**Config**
* `listen_address`: Configures the IP and Port the server listens on. Format `ip_address:port`, defaults to `:8080`.
* `disable_registration`: Rejects registration requests when enabled, defaults to `false`.
* `enable_debug_log`: Enables verbose logging for debugging
* `store_history`: Enables storing historic records for each file
* `backup_encoding_type`: Specifies the content-type used for the PEM backup file, defaults to `msgpack` (available are `json` and `msgpack`)

**Users**
* `<username>`: The name provided during register in KOReader and used for login
* `<password>`: The password entered into KOReader hashed with MD5 in KOReader itself

**Documents**
* `<filehash>`: Determined by KOReader, defaults to MD5 hash of the read file
* `percentage`: Progress as number between 0 and 1 (so `0.1` is 10 percent)
* `progress`: URI within position inside of the file
* `device`: Name of the KOReader device
* `device_id`: Unique ID of the KOReader device
* `timestamp`: Unix Timestamp when the progress update was recieved by the server

**History** (when `store_history` is enabled, otherwise empty as `{}`)
* `<filehash>`: Same as `Documents.<filehash>`
* `document_history`: Array of `Documents[]` objects sorted from oldest to newest

### Backup Files

When a newer version of KOsync is started which changes the database.json schema,  
the server automatically creates a backup file `.bak`.

These backup files are a bit special, instead of just copying the database file,  
they are PEM encoded files.

I have chosen to use PEM for three reasons:

- fun. (just though it would be funny)
- Metadata, PEM has the feature of "Headers"
- Alternative encodings (like [msgpack](https://msgpack.io))

Backup files can be restored by adding the `--restore <path/to/database.bak>` command line option.  
The server will try to restore the database and then start on success.

## License

KOsync is licensed under the [European Union Public License v1.2 or later](/LICENSE.txt)
