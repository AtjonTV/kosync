# JMP - JSON Message Protocol

JMP is a simple JSON Object messaging protocol for RPC and PubSub.

## Protocol v1 Format Specification
Each message must be a JSON Object with all the following fields:
- `jmp: string`: Version of the JMP Protocol. Must be `1`
- `proto: string`: Name of Sub-Protocol. One of `rpc` or `pubsub`
- `content: string`: Protocol content-type hint. One of (rpc.call, rpc.result, pubsub.subscribe, pubsub.subscription, pubsub.announce)
- `payload: object`: Payload data of the specified protocol content-type.

### RPC
RPC Request Payload
- method: Name of the RPC to call

RPC Response Payload:
- for_rpc: Name of the RPC (same as "method" from the request)
- type_hint: Optional string to identify the Result Object type
- data: Object with RPC specific data. Must be null on error.
- errors: Array of Strings in case any errors occurred. Must be empty when non occurred

### PubSub
PubSub Request Payload
- topic: Name of the Topic to subscribe to

PubSub Response Payload
- for_topic: Name of the topic (same as "topic" from the request)
- type_hint: Hardcoded "SubscriptionResult"
- data: Object with "subscribed" boolen field. Must be null on error.
- errors: Array of Strings in case any errors occurred. Must be empty when non occurred

PubSub Announce Payload
- for_topic: Name of the topic
- type_hint: Optional string to identify the PubSub Object type
- data: Object with PubSub specific data. Must be null on error.
- errors: Array of Strings in case any errors occurred. Must be empty when non occurred


Type 'SubscriptionResult'
- subscribed: Boolean status of PubSub Subscription
