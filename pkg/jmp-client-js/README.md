# JMP Client: TypeScript WebSocket Client for JMP Protocol

A lightweight TypeScript WebSocket client for sending and receiving JMP protocol messages (RPC and PubSub) using the standard WebSocket API.

## Installation

Add the client as a local dependency in your project:

```bash
bun add jmp-client-js
```

Or add it to your `package.json`:

```json
"dependencies": {
  "jmp-client-js": "^0.1.0"
}
```

## Usage

### Import the Client
```typescript
import JMPClient from 'jmp-client-js';
```

### Instantiation
```typescript
const client = new JMPClient("ws://example.com");
client.connect();
```

### Subscribe to a Topic
```typescript
client.subscribe("user.documents", (data) => {
  console.log("Received data:", data);
});
```

### Send RPC Requests
```typescript
client.rpc("getData", { arg1: "value" }).then((data) => {
  console.log("Response:", data);
});
```

## Protocol Reference

Each message must follow the JMP protocol format:

```json
{
  "jmp": "1",
  "proto": "rpc/pubsub",
  "content": "rpc.call/pubsub.subscribe",
  "payload": { ... }
}
```

### Example Message
For subscribing to a topic:
```json
{
  "jmp": "1",
  "proto": "pubsub",
  "content": "pubsub.subscribe",
  "payload": {
    "topic": "user.documents"
  }
}
```

## Error Handling

- **WebSocket Errors**: Handle connection errors using the `onerror` event.
- **Message Parsing**: Validate message structure with error handling:
  ```typescript
  try {
    const msg = JSON.parse(message);
    if (msg.proto !== "pubsub") return;
  } catch (error) {
    console.error("Invalid message:", error);
  }
  ```

## Example Integration

For integrating with a project like `webui`:
```typescript
import { getWebSocketUrl } from '@/api.ts';

const client = new JMPClient(getWebSocketUrl());
client.connect();
client.subscribe("user.documents", (data) => {
  // Update sync.value as needed
});
```
