# Configuration

KOsync can be configured via environment variables.  
They can either be set in the environment or in a `kosync.env` file that is placed besides the KOsync executable.

Here is a list of configuration variables currently supported:
- `DATABASE_FILE`: Path to the SQLite database. Default `./kosync.db`
- `LISTEN_ADDRESS`: Address to listen on. Default `:8080`
- `DEBUG_LOG`: Enable debug logging. Default `false`
- `DISABLE_REGISTRATION`: Disable user registration via KOReader. Default `false`
- `ENABLE_WEBUI`: Enable the web UI if available¹. Default `false`
- `ENABLE_TRUSTED_PROXIES`: Enable support for trusted proxies. Default `false`
- `TRUSTED_PROXIES`: Comma separated list of trusted proxies. Default `""`
- `PROXY_IP_VALIDATION`: Enable IP validation for trusted proxies. Default `false`
- `LOG_TO_FILE`: Write logs to file. Default `false`
- `LOG_FILE`: Path to the log file. Default `./kosync.log`
- `ENABLE_WEBSOCKET_API`: Enable the experimental WebSocket API. Default `false`
- `PRINT_CRYPTO_KEYS`: Print crypto keys used for JWT to stdout. Default `false`
- `CRYPTO_KEYS_SEED`: Seed for the crypto keys. Must be 32 Characters log. Default `""` (random)

¹: Requires a KOsync executable with the WebUI compiled in. See the [build](./build.md) documentation.

Example `kosync.env` with common configuration:

```
# Only available on current machine instead of all networks
LISTEN_ADDRESS=127.0.0.1:8080

# Allow users to register
DISABLE_REGISTRATION=false

# Enable the web UI so users can view their progress
ENABLE_WEBUI=true
ENABLE_WEBSOCKET_API=true

# Write logs to file
LOG_TO_FILE=true
```

### Overrides
The way KOsync is currently configured, the environment used to start KOsync overrides the configuration file.
