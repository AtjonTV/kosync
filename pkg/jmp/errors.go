package jmp

import "errors"

var ErrUnsupportedJmpVersion = errors.New("the JMP message has an unsupported version")
var ErrRpcMethodAlreadyHasHandler = errors.New("the rpc method already has a handler registered")
var ErrRpcExpectedRpcCall = errors.New("expected an JMP message with Proto=RPC and Content=rpc.call")
var ErrRpcCouldNotFindMethodHandler = errors.New("the JMP message could not be processed as no RPC handler was found")
