# KOsync

KOsync is a progress sync server for KOReader written in less than 400 lines of Go.

## Goals

The official KOReader sync server needs Nginx, OpenResty, Lua and Redis to run.  
Getting rid of external dependencies is a major goal.

Another goal is simplicity, make it as simple as possible.  
Both with code simplicity as well as deployment simplicity.

### Simplicity

KOsync is written in Go and only depends on the standard library.

Compilation only requires the Go Toolchain and this command `go build -tags netgo main.go`.  
The command compiles the main.go file to a single static binary.

KOsync stores all data, both configuration and user data, in a single JSON file.

File structure:
```json
{
  "listen_address": "<listen_address>",
  "disable_registration": false,
  "users": {
    "<username>": {
      "username": "<username>",
      "password": "<password>",
      "documents": {
        "<filehash>": {
          "percentage": 0,
          "progress": 0,
          "device": "<device>",
          "device_id": "<device_id>",
          "timestamp": 0
        }
      }
    }
  }
}
```

Each string in `<>` is dynamic.

* `listen_address`: Configures the IP and Port the server listens on. Format `ip_address:port`, defaults to `:8080`.
* `disable_registration`: Rejects registration requests when enabled, defaults to `false`.

The server will create all other configuration data.

The Users are created after `/users/create` is called via the Registration feature in the client.  
The Documents are created after the client pushes the progress of a document.

## License

KOsync is licensed under the [European Union Public License v1.2 or later](/LICENSE.txt)
