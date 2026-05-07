//
// File:        pkg/jmp/types_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package jmp

import (
	"errors"
	"testing"
)

func TestNewContext(t *testing.T) {
	ctx := NewContext()
	if ctx == nil {
		t.Fatal("NewContext() returned nil")
	}
	if ctx.UniqueRequestorId == 0 {
		t.Error("UniqueRequestorId should not be 0")
	}
	if ctx.Data == nil {
		t.Error("Data map should be initialized")
	}
}

func TestContext_HasId(t *testing.T) {
	ctx := &Context{UniqueRequestorId: 123}
	if !ctx.HasId() {
		t.Error("Expected HasId() to be true for non-zero ID")
	}
	ctx.UniqueRequestorId = 0
	if ctx.HasId() {
		t.Error("Expected HasId() to be false for zero ID")
	}
}

func TestContext_GetString(t *testing.T) {
	ctx := NewContext()
	ctx.Data["name"] = "Junie"
	ctx.Data["age"] = 1 // Not a string

	if val := ctx.GetString("name"); val != "Junie" {
		t.Errorf("Expected 'Junie', got '%s'", val)
	}
	if val := ctx.GetString("age"); val != "" {
		t.Errorf("Expected empty string for non-string type, got '%s'", val)
	}
	if val := ctx.GetString("missing"); val != "" {
		t.Errorf("Expected empty string for missing key, got '%s'", val)
	}
}

func TestNewRpcNoticeMessage(t *testing.T) {
	payload := RpcResponsePayload{ForRpc: "test"}
	msg := NewRpcNoticeMessage(payload)
	if msg.Content != RpcNotice {
		t.Errorf("Expected %s, got %s", RpcNotice, msg.Content)
	}
	if msg.Proto != ProtoRpc {
		t.Errorf("Expected %s, got %s", ProtoRpc, msg.Proto)
	}
}

func TestNewRpcResultMessage(t *testing.T) {
	payload := RpcResponsePayload{ForRpc: "test"}
	msg := NewRpcResultMessage(payload)
	if msg.Content != RpcResult {
		t.Errorf("Expected %s, got %s", RpcResult, msg.Content)
	}
}

func TestNewRpcResultFromError(t *testing.T) {
	err := errors.New("something went wrong")
	msg := NewRpcResultFromError("someMethod", err)
	if msg.Content != RpcResult {
		t.Errorf("Expected %s, got %s", RpcResult, msg.Content)
	}
	payload := msg.Payload.(RpcResponsePayload)
	if payload.ForRpc != "someMethod" {
		t.Errorf("Expected someMethod, got %s", payload.ForRpc)
	}
	if payload.Result.Errors[0] != "something went wrong" {
		t.Errorf("Expected error message, got %v", payload.Result.Errors)
	}
}

func TestMessage_IsRpcCall(t *testing.T) {
	msg := &Message{Proto: ProtoRpc, Content: RpcCall}
	if !msg.IsRpcCall() {
		t.Error("Expected IsRpcCall() to be true")
	}
}

func TestMessage_IsPubSubSubscribe(t *testing.T) {
	msg := &Message{Proto: ProtoPubSub, Content: PubSubscribe}
	if !msg.IsPubSubSubscribe() {
		t.Error("Expected IsPubSubSubscribe() to be true")
	}
}

func TestNewOkResult(t *testing.T) {
	res := NewOkResult("string", "hello")
	if res.TypeHint != "string" {
		t.Errorf("Expected string, got %s", res.TypeHint)
	}
	if res.Data != "hello" {
		t.Errorf("Expected hello, got %v", res.Data)
	}
	if len(res.Errors) != 0 {
		t.Errorf("Expected no errors, got %d", len(res.Errors))
	}
}

func TestNewErrorResult(t *testing.T) {
	errs := []string{"err1", "err2"}
	res := NewErrorResult(errs)
	if res.TypeHint != TypeErrors {
		t.Errorf("Expected %s, got %s", TypeErrors, res.TypeHint)
	}
	if len(res.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(res.Errors))
	}
}

func TestMessageFromMap(t *testing.T) {
	data := map[string]any{
		"jmp":     "1",
		"proto":   "rpc",
		"content": "rpc.call",
		"payload": map[string]any{"method": "test"},
	}
	msg, err := MessageFromMap(data)
	if err != nil {
		t.Fatalf("MessageFromMap failed: %v", err)
	}
	if msg.Version != "1" {
		t.Errorf("Expected version 1, got %s", msg.Version)
	}
}

func TestNewRpcResponseFromResult(t *testing.T) {
	res := NewOkResult("int", 42)
	resp := NewRpcResponseFromResult("method", res)
	if resp.ForRpc != "method" {
		t.Errorf("Expected method, got %s", resp.ForRpc)
	}
	if resp.Result.Data != 42 {
		t.Errorf("Expected 42, got %v", resp.Result.Data)
	}
}
