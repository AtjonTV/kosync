package jmp

// Version is the implemented protocol version
const Version = "1"

// JMP 'JSON Message Protocol' - a JSON Based RPC and PubSub Protocol
type JMP struct {
	rpcHandlers map[string]RpcHandler
}

func New() *JMP {
	return &JMP{
		rpcHandlers: make(map[string]RpcHandler),
	}
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

func (s *JMP) HandleRpc(ctx *Context, msg *Message) (*Message, error) {
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
