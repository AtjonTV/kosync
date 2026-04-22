# WebSocket API

KOsync has a JMP based WebSocket API that can be used for RPC and PubSub.

The WebSocket API is currently experimental and `ENABLE_WEBSOCKET_API` has to be set to `true` in order to use it.

You can read about JMP (JSON Message Protocol), like how messages are formatted, in [`pkg/jmp/README.md` (here)](../pkg/jmp/README.md).

## Example communication

```json5
// Server hello
< {"jmp": "1", "proto": "rpc", "content": "rpc.notice", "payload": {"for_rpc": "", "result": {"type_hint": "ServerInfo", "data": {"server_name": "KOsync", "server_version": "2026.06.1-dev.3", "message": "KOsync WebSocket API. Hello!"}}}}

// Client subscribe to user "documents"
> {"jmp": "1", "proto": "pubsub", "content": "pubsub.subscribe", "payload": {"topic": "user.documents"}}
< {"jmp": "1", "proto": "pubsub", "content": "pubsub.subscription", "payload": {"for_topic": "user.documents", "result": {"type_hint": "SubscriptionResult", "data": {"subscribed": true}}}}

// Server sends announces
< {"jmp": "1", "proto": "pubsub", "content": "pubsub.announce", "payload": {"for_topic": "user.documents", "result": {"type_hint": "Document", "data": {"id":"043f11771ef9d191364ac0ba08198d36","progress":"/body/DocFragment[1]/body/div/svg.0","percentage":0.0033,"device":"Flatpak","device_id":"BDD3C5BCA1624FE996EB00FC7948468E","document":"043f11771ef9d191364ac0ba08198d36","timestamp":1770653038,"pretty_name":"","history":null}}}}

// Client asks for disconnect
> {"jmp": "1", "proto": "rpc", "content": "rpc.call", "payload": {"method": "disconnect"}}
< {"jmp": "1", "proto": "rpc", "content": "rpc.result", "payload": {"for_rpc": "disconnect", "result": {"type_hint": "string", "data": "goodbye."}}}
```

* `<` Sent from server
* `>` Sent from client

### RPC

#### Known RPC methods

These are the currently known RPC methods:
- `documents.all`: Get all documents of the current user. Returns the same list as `/api/documents.all`.
- `documents.update`: Update a document. Argument `document` must given. Returns the updated document.
- `disconnect`: Disconnect the WebSocket connection. Returns `goodbye.` on success.

### PubSub

#### Known topics

These are the currently known topics:
- `user.documents`: Get announces when a single document of the current user changes.

## Authentication

The WebSocket API is protected by a JWT.

To get a WebSocket URI with user credentials, send a request to `/api/ws` with the following headers:
- `x-auth-user`: Username of the user to authenticate
- `x-auth-key`: Password hash of the user (must be MD5 because that is what KOReader uses)

This will return a full `wss://` (or `ws://`) URI that can be directly shoved into a WebSocket library.  

Alternatively, you can send the same request to `/api/auth.jwt` to get a JWT in the response body that can be used in the `Authorization` header for any API endpoint.

With an existing JWT, construct the URI `wss://kosync.host.internal/api/ws/{token}` to connect to the WebSocket API.
