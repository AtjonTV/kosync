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

The Users are created after `/users/create` is called via the Registration feature in the client.  
The Documents are created after the client pushes the progress of a document.

### Database File

Example:
```json
{
  "schema": 1,
  "config": {
    "listen_address": ":8080",
    "disable_registration": false,
    "enable_debug_log": false,
    "store_history": false
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

## License

KOsync is licensed under the [European Union Public License v1.2 or later](/LICENSE.txt)
