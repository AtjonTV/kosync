package jmp

import "slices"

// Version is the implemented protocol version
const Version = "1"

// JMP 'JSON Message Protocol' - a JSON Based RPC and PubSub Protocol
type JMP struct {
	rpcHandlers     map[string]RpcHandler
	pubSubWriters   map[string]PubSubWriter
	pubSubListeners map[string]*[]PubSubSubscription
	knownTopics     *[]string
}

func New() *JMP {
	return &JMP{
		rpcHandlers:     make(map[string]RpcHandler),
		pubSubWriters:   make(map[string]PubSubWriter),
		pubSubListeners: make(map[string]*[]PubSubSubscription),
		knownTopics:     new([]string),
	}
}

func (s *JMP) HandleMessage(ctx *Context, message *Message) (*Message, error) {
	if message.IsRpcCall() {
		return s.handleRpc(ctx, message)
	} else if message.IsPubSubSubscribe() {
		return s.handlePubSubSubscription(ctx, message)
	}

	return NewRpcResultFromError("", ErrUnsupportedJmpProto), ErrUnsupportedJmpProto
}

type RpcHandler func(ctx *Context, payload *RpcRequestPayload) Result

func (s *JMP) RegisterRpc(name string, handler RpcHandler) error {
	_, found := s.rpcHandlers[name]
	if found {
		return ErrRpcMethodAlreadyHasHandler
	}

	s.rpcHandlers[name] = handler
	return nil
}

func (s *JMP) UnregisterRpc(name string) {
	delete(s.rpcHandlers, name)
}

func (s *JMP) handleRpc(ctx *Context, msg *Message) (*Message, error) {
	if msg.Version != Version {
		return nil, ErrUnsupportedJmpVersion
	}
	if !msg.IsRpcCall() {
		return NewRpcResultFromError("", ErrRpcExpectedRpcCall), ErrRpcExpectedRpcCall
	}

	payload, err := MessagePayloadAs[RpcRequestPayload](msg)
	if err != nil {
		return NewRpcResultFromError("", err), err
	}

	handler, found := s.rpcHandlers[payload.Method]
	if !found {
		return NewRpcResultFromError(payload.Method, ErrRpcCouldNotFindMethodHandler), ErrRpcCouldNotFindMethodHandler
	}

	result := handler(ctx, payload)
	response := Message{
		Version: Version,
		Proto:   ProtoRpc,
		Content: RpcResult,
		Payload: RpcResponsePayload{
			ForRpc: payload.Method,
			Result: result,
		},
	}
	return &response, nil
}

// PubSubWriter is a function that sends the riven method to the contexts underlying socket
type PubSubWriter func(ctx *Context, msg *Message)

func (s *JMP) RegisterPubSubWriter(writerName string, handler PubSubWriter) error {
	_, found := s.pubSubWriters[writerName]
	if found {
		return ErrPubSubWriterAlreadyRegistered
	}

	s.pubSubWriters[writerName] = handler
	return nil
}

func (s *JMP) RegisterKnownTopic(topic string) error {
	found := slices.Contains(*s.knownTopics, topic)
	if found {
		return ErrPubSubTopicAlreadyKnown
	}
	*s.knownTopics = append(*s.knownTopics, topic)
	return nil
}

func (s *JMP) handlePubSubSubscription(ctx *Context, msg *Message) (*Message, error) {
	if msg.Version != Version {
		return nil, ErrUnsupportedJmpVersion
	}
	if !msg.IsPubSubSubscribe() {
		return NewRpcResultFromError("", ErrRpcExpectedRpcCall), ErrRpcExpectedRpcCall
	}

	payload, err := MessagePayloadAs[PubSubscribePayload](msg)
	if err != nil {
		return NewRpcResultFromError("", err), err
	}

	if !ctx.HasId() {
		return nil, ErrPubSubContextMissingId
	}

	subs, found := s.pubSubListeners[payload.Topic]
	if !found {
		subs = new([]PubSubSubscription)
		s.pubSubListeners[payload.Topic] = subs
	}

	*subs = append(*subs, PubSubSubscription{
		Ctx:   ctx,
		Topic: payload.Topic,
	})

	response := Message{
		Version: Version,
		Proto:   ProtoRpc,
		Content: PubSubscription,
		Payload: PubSubResponsePayload{
			ForTopic: payload.Topic,
			Result: Result{
				TypeHint: "SubscriptionResult",
				Data: PubSubSubscriptionResult{
					Subscribed: true,
				},
			},
		},
	}
	return &response, nil
}

func (s *JMP) UnregisterPubSubHandler(writerName string) {
	delete(s.pubSubWriters, writerName)
}

func (s *JMP) UnregisterKnownTopic(topic string) error {
	found := slices.Contains(*s.knownTopics, topic)
	if !found {
		return ErrPubSubTopicUnknown
	}
	*s.knownTopics = slices.DeleteFunc(*s.knownTopics, func(t string) bool {
		return t == topic
	})
	return nil
}

func (s *JMP) PubSubAnnounce(topic string, recipient *int64, data any, typeHint string) error {
	found := slices.Contains(*s.knownTopics, topic)
	if !found {
		return ErrPubSubTopicUnknown
	}

	subs, found := s.pubSubListeners[topic]
	if !found {
		return ErrPubSubNoListenersForTopic
	}

	payload := Message{
		Version: Version,
		Proto:   ProtoRpc,
		Content: PubAnnounce,
		Payload: PubSubResponsePayload{
			ForTopic: topic,
			Result: Result{
				TypeHint: typeHint,
				Data:     data,
			},
		},
	}

	for _, write := range s.pubSubWriters {
		for _, sub := range *subs {
			// TODO: Find out if this is slow
			if recipient != nil {
				if sub.Ctx.UniqueRequestorId == *recipient {
					write(sub.Ctx, &payload)
					break
				}
				continue
			}
			write(sub.Ctx, &payload)
		}
	}
	return nil
}
