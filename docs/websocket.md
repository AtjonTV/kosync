# WebSocket API

```json5
// Server hello
{"type":"connected","payload":"Welcome. KOsync WebSocket API"}

// Client subscribe to user "documents"
{"type": "pubsub", "payload": {"topic": "user.documents"}}
{"type":"pubsub","payload":{"for_topic":"user.documents","data":"subscribed"}}

// Server sends announces
{"type":"pubsub","payload":{"for_topic":"user.documents","data":{"id":"043f11771ef9d191364ac0ba08198d36","progress":"/body/DocFragment[1]/body/div/svg.0","percentage":0.0033,"device":"Flatpak","device_id":"BDD3C5BCA1624FE996EB00FC7948468E","document":"043f11771ef9d191364ac0ba08198d36","timestamp":1770653038,"pretty_name":"","history":null}}}

// Client asks for disconnect
{"type": "rpc", "payload": {"method": "disconnect"}}
{"type": "rpc","payload":{"for_rpc":"disconnect","result":"goodbye.","error":""}}
```

* `<` Sent from server
* `>` Sent from client

## Message Format

All messages are inside the Container, so each JSON object must have a type and payload.

### Message Container
- type: What type of message is this?
- payload: JSON Data for type needed to request

### Message Types
There are two different "types":

- rpc
- pubsub

### Request Payload

If the message is sent as a request, "type" must be one of the know message types,  
and the payload must be one of the following:

#### RPC Format
- method: Name of method
- arguments: List of arguments (positional)

#### PubSub Format
- topic: Name of the topic (user.documents)

### Response Payload

If the message is sent as a response, "type" must be the same as of the request,  
and the payload must be one of the following according to the type:

#### RPC Format
- for_rpc: Name of RPC Method
- data: JSON Data of rpc result
- error: String failure message

#### PubSub Format
- for_topic: Name of RPC Method
- data: JSON Data of rpc result
- error: String failure message
