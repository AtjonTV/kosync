# WebSocket API

KOsync has a WebSocket API that can be used for RPC and PubSub.

The WebSocket API is currently experimental and `ENABLE_WEBSOCKET_API` has to be set to `true` in order to use it.

## Example communication

```json5
// Server hello
< {"type": "rpc", "payload": {"server_name": "KOsync", "server_version": "2026.05.1-dev.5", "message":"KOsync WebSocket API. Hello!"}}

// Client subscribe to user "documents"
> {"type": "pubsub", "payload": {"topic": "user.documents"}}
< {"type": "pubsub", "payload": {"for_topic": "user.documents", "data": "subscribed"}}

// Server sends announces
< {"type": "pubsub", "payload": {"for_topic": "user.documents", "data": {"id":"043f11771ef9d191364ac0ba08198d36","progress":"/body/DocFragment[1]/body/div/svg.0","percentage":0.0033,"device":"Flatpak","device_id":"BDD3C5BCA1624FE996EB00FC7948468E","document":"043f11771ef9d191364ac0ba08198d36","timestamp":1770653038,"pretty_name":"","history":null}}}

// Client asks for disconnect
> {"type": "rpc", "payload": {"method": "disconnect"}}
< {"type": "rpc", "payload": {"for_rpc": "disconnect", "result": "goodbye.", "error": ""}}
```

* `<` Sent from server
* `>` Sent from client

## Message Format

All messages are inside a container object.  
The container object has two properties:
- type: What type of message is this?
- payload: JSON Data for type needed to request

There are two supported types:
- rpc: Remote function calling
- pubsub: Async message broadcasting

Unless `(optional)` is specified, all properties are required.

### RPC

To call a remote function, the `type` must be `rpc` and the `payload` must be an object with the following properties:
- method: Name of the method to call
- arguments: (optional) Object with properties

The server will respond with a message of type `rpc` and a `payload` with the following properties:
- for_rpc: Name of RPC Method
- result: JSON Data of rpc result
- error: String failure message

#### Known RPC methods

These are the currently known RPC methods:
- `documents.all`: Get all documents of the current user. Returns the same list as `/api/documents.all`.
- `disconnect`: Disconnect the WebSocket connection. Returns `goodbye.` on success.

### PubSub
To subscribe to a topic, the `type` must be `pubsub` and the `payload` must be an object with the following properties:
- topic: Name of the topic to subscribe to

The server will respond with a message of type `pubsub` and a `payload` with the following properties:
- for_topic: Same topic as in subscribe request
- data: JSON Data of an announcement or the text `subscribed`
- error: String failure message

The first response will have the string `subscribed` in the data property.  

Following the first response, the server sends asynchronous announces for the topic.  
The `data` of these announces depend on the topic.

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
