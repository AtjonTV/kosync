# Database

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
