//
// File:        pkg/jmp/types.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package jmp

import (
	"math/rand"
	"reflect"

	"git.obth.eu/atjontv/kosync/pkg/decode"
)

const ProtoRpc = "rpc"
const ProtoPubSub = "pubsub"

const RpcCall = "rpc.call"
const RpcResult = "rpc.result"
const RpcNotice = "rpc.notice"

const PubSubscribe = "pubsub.subscribe"
const PubSubscription = "pubsub.subscription"
const PubAnnounce = "pubsub.announce"

const TypeString = "string"
const TypeInt = "int"
const TypeFloat = "float"
const TypeBool = "bool"
const TypeMap = "map"
const TypeArray = "array"
const TypeErrors = "errors"

type Payload any

type Message struct {
	Version  string  `json:"jmp"`                // Version is the JMP Protocol version
	Proto    string  `json:"proto"`              // Proto is the JMP Sub-Protocol (rpc or pubsub)
	Content  string  `json:"content"`            // Content is a Protocol-specific type identifier (like rpc.call)
	Payload  Payload `json:"payload"`            // Payload is the request or response payload
	Sequence int64 `json:"sequence,omitempty"` // Sequence is a client given and server repeated message sequence number
}

func MessageFromMap(data map[string]any) (*Message, error) {
	msg := new(Message)
	err := decode.StructFromMap(msg, "json", data)
	return msg, err
}

func MessagePayloadAs[X any](msg *Message) (*X, error) {
	var res X
	err := decode.StructFromMap(&res, "json", msg.Payload.(map[string]interface{}))
	return &res, err
}

func NewRpcNoticeMessage(payload RpcResponsePayload) *Message {
	return &Message{
		Version: Version,
		Proto:   ProtoRpc,
		Content: RpcNotice,
		Payload: payload,
	}
}

func NewRpcResultMessage(payload RpcResponsePayload) *Message {
	return &Message{
		Version: Version,
		Proto:   ProtoRpc,
		Content: RpcResult,
		Payload: payload,
	}
}

func NewRpcResultFromError(methodName string, err error) *Message {
	return NewRpcResultMessage(NewRpcResponseFromResult(methodName, NewErrorResultFromErr(err)))
}

func (m Message) IsRpcCall() bool {
	return m.Proto == ProtoRpc && m.Content == RpcCall
}

func (m Message) IsPubSubSubscribe() bool {
	return m.Proto == ProtoPubSub && m.Content == PubSubscribe
}

type Context struct {
	UniqueRequestorId int64
	Data              map[string]any
	RawSocket         any
}

func NewContext() *Context {
	return &Context{
		// This random value is not for a secure context, it is only used to identify a client later on for sending responses.
		// bearer:disable go_gosec_crypto_weak_random
		UniqueRequestorId: rand.Int63(),
		Data:              make(map[string]any),
	}
}

func (c Context) HasId() bool {
	return c.UniqueRequestorId != 0
}

func (c Context) GetString(key string) string {
	val, found := c.Data[key]
	if !found || reflect.TypeOf(val).Kind() != reflect.String {
		return ""
	}
	return val.(string)
}

type Result struct {
	TypeHint string   `json:"type_hint"` // TypeHint is the name of the struct of Data
	Data     any      `json:"data"`      // Data is some response data
	Errors   []string `json:"errors"`    // Errors is a list of errors
}

func NewOkResult(typeHint string, data any) Result {
	return Result{
		TypeHint: typeHint,
		Data:     data,
		Errors:   []string{},
	}
}

func NewErrorResult(errors []string) Result {
	return Result{
		TypeHint: TypeErrors,
		Data:     nil,
		Errors:   errors,
	}
}

func NewErrorResultFromErr(err error) Result {
	return NewErrorResult([]string{err.Error()})
}

type RpcRequestPayload struct {
	Method    string         `json:"method"`
	Arguments map[string]any `json:"arguments"`
}

type RpcResponsePayload struct {
	ForRpc string `json:"for_rpc"` // ForRpc is the Method name from RpcRequestPayload
	Result
}

func NewRpcResponseFromResult(methodName string, result Result) RpcResponsePayload {
	return RpcResponsePayload{
		ForRpc: methodName,
		Result: result,
	}
}

type PubSubscribePayload struct {
	Topic string `json:"topic"` // Topic is the PubSub topic name
}

type PubSubResponsePayload struct {
	ForTopic string `json:"for_topic"` // ForTopic is the Topic name from PubSubscribePayload
	Result
}

type PubSubSubscriptionResult struct {
	Subscribed bool `json:"subscribed"`
}

type PubSubSubscription struct {
	Ctx   *Context
	Topic string
}
