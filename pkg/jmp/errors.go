package jmp

import "errors"

var ErrUnsupportedJmpVersion = errors.New("the JMP message has an unsupported version")
var ErrUnsupportedJmpProto = errors.New("the JMP message has an unsupported protocol")

var ErrRpcMethodAlreadyHasHandler = errors.New("the rpc method already has a handler registered")
var ErrRpcExpectedRpcCall = errors.New("expected an JMP message with Proto=RPC and Content=rpc.call")
var ErrRpcCouldNotFindMethodHandler = errors.New("the JMP message could not be processed as no RPC handler was found")

var ErrPubSubTopicAlreadyKnown = errors.New("the pubsub topic is already known")
var ErrPubSubTopicUnknown = errors.New("the pubsub topic is not yet known, please add it to the known topics")
var ErrPubSubContextMissingId = errors.New("the context used to handle the pubsub subscription is missing an id")

var ErrPubSubNoListenersForTopic = errors.New("there are no pubsub listeners for this topic")
var ErrPubSubWriterAlreadyRegistered = errors.New("a pubsub writer with this name is already registered")
