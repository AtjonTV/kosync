# KOsync

KOsync is a progress sync server for KOReader written in Go.

## Why?

The [official KOReader progress sync server](https://github.com/koreader/koreader-sync-server) is written in Lua using OpenResty.  
For deployment it needs Nginx with OpenResty as well as Redis as database.

KOsync wants to be simpler by not having any dependencies besides the OS itself.  
(If you need TLS, a reverse proxy is also required, I recommend [Caddy](https://caddyserver.com))

In addition to requiring Nginx, OpenResty and Redis, the official server is not very maintained.  
The last feature adding commits was around 2016.

## KOsync vs [KOReader Sync Server](https://github.com/koreader/koreader-sync-server)

You may choose KOsync over [KORSS](https://github.com/koreader/koreader-sync-server) due to the following differences:

- Actively maintained and open for feature requests
- Simple [Web Interface](webui/README.md) (Prototype)
- Written in Go and deploys as a single executable
- Simple SQLite database and ENV configuration instead of Redis

Additional differences that should be known:

- KOsync is licensed under `EUPL-1.2 or later` compared to KORSS, which is `AGPL-3.0 or later`
- Simple deployment via Docker
- Requires a Reverse Proxy for TLS

### Simplicity

**Simple Code**  
KOsync is written in Go with no external dependencies.  
All you need to run KOsync is bundled into a single executable.

See [docs/build.md](docs/build.md) for build and deployment instructions.

**Simple Datastore**  
KOsync stores all user data in an SQLite database while configuration is stored in an environment file.

Users can, after entering the custom URL, use the KOReader registration to signup.  
After that, they push and pull progress states.

Documents are uploaded by KOReader during progress push.  
The push must be triggered by hand or configured to be done automatically when switching pages.  
Consult the KOReader documentation for the configuration options.

### Configuration

See [docs/config.md](docs/config.md)

### Database

See [docs/database.md](docs/database.md)

### Backups

Backup and Restore can be done manually or with automation tools made for SQLite.  
Stop KOsync and copy the database file to create a backup or replace the database to restore.

In the future KOsync will provide a backup and restore mechanism.

KOsync uses [modernc sqlite](https://pkg.go.dev/modernc.org/sqlite#Backup) which supports backup and restore natively.

(Code Example of the API for later reference)
```go
type SQLiteBackuper interface {
    NewBackup(string) (*sqlite.Backup, error)
    NewRestore(string) (*sqlite.Backup, error)
}

c, _ := db.Conn(context.Background())
c.Raw(func(driverConn any) error {
    bak, err := driverConn.(SQLiteBackuper).NewBackup("pathToBackupTo.db")
    bakCon, err := bak.Commit()
    err = bak.Finish()
})
```

### API Specification

See [docs/api.md](docs/api.md) for REST-like.  
See [docs/websocket.md](docs/websocket.md) for RPC/PubSub-like.

### WebUI

See [webui/README.md](webui/README.md)

## License

KOsync is licensed under the [European Union Public License v1.2 or later](/LICENSE.txt)
