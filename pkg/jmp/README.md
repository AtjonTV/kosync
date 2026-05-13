# JMP - JSON Message Protocol

JMP is a simple JSON Object messaging protocol for RPC and PubSub.

## Protocol v1 Format Specification
Each message must be a JSON Object with all the following fields:
- `jmp: string`: Version of the JMP Protocol. Must be `1`
- `proto: string`: Name of Sub-Protocol. One of `rpc` or `pubsub`
- `content: string`: Protocol content-type hint. One of (rpc.call, rpc.result, pubsub.subscribe, pubsub.subscription, pubsub.announce)
- `payload: object`: Payload data of the specified protocol content-type.
- `sequence: number`: (Optional) Client-given sequence number. (The server will respond with the same sequence number)

### RPC
RPC Request Payload
- `method: string`: Name of the RPC to call
- `arguments: object`: Object with RPC specific arguments, might be null

RPC Response Payload:
- `for_rpc: string`: Name of the RPC (same as "method" from the request)
- `type_hint: string`: Optional string to identify the Result Object type
- `data: object`: Object with RPC specific data. Must be null on error.
- `errors: string[]`: Array of Strings in case any errors occurred. Must be empty when non occurred

### PubSub
PubSub Request Payload
- `topic: string`: Name of the Topic to subscribe to

PubSub Response Payload
- `for_topic: string`: Name of the topic (same as "topic" from the request)
- `type_hint: string`: Hardcoded "SubscriptionResult"
- `data: object`: Object with "subscribed" boolen field. Must be null on error.
- `errors: string[]`: Array of Strings in case any errors occurred. Must be empty when non occurred

PubSub Announce Payload
- `for_topic: string`: Name of the topic
- `type_hint: string`: Optional string to identify the PubSub Object type
- `data: object`: Object with PubSub specific data. Must be null on error.
- `errors: string[]`: Array of Strings in case any errors occurred. Must be empty when non occurred

#### Predefined Types

Type `SubscriptionResult`
- `subscribed: bool`: Boolean status of PubSub Subscription
