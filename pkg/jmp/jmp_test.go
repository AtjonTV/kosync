//
// File:        pkg/jmp/jmp_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package jmp

import (
	"fmt"
	"testing"
)

func TestNew(t *testing.T) {
	s := New()
	if s == nil {
		t.Fatal("New() returned nil")
	}
}

func TestJMP_RegisterRpc(t *testing.T) {
	s := New()
	handler := func(ctx *Context, req *RpcRequestPayload) Result {
		return NewOkResult("string", "hello")
	}
	err := s.RegisterRpc("test", handler)
	if err != nil {
		t.Errorf("RegisterRpc failed: %v", err)
	}
	err = s.RegisterRpc("test", handler)
	if err == nil {
		t.Error("RegisterRpc should fail for duplicate registration")
	}
}

func TestJMP_UnregisterRpc(t *testing.T) {
	s := New()
	s.RegisterRpc("test", func(ctx *Context, req *RpcRequestPayload) Result { return Result{} })
	s.UnregisterRpc("test")
	if _, ok := s.rpcHandlers["test"]; ok {
		t.Error("RpcHandler still exists after UnregisterRpc")
	}
}

func TestJMP_RegisterKnownTopic(t *testing.T) {
	s := New()
	err := s.RegisterKnownTopic("topic1")
	if err != nil {
		t.Errorf("RegisterKnownTopic failed: %v", err)
	}
	err = s.RegisterKnownTopic("topic1")
	if err == nil {
		t.Error("RegisterKnownTopic should fail for duplicate topic")
	}
}

func TestJMP_UnregisterKnownTopic(t *testing.T) {
	s := New()
	s.RegisterKnownTopic("topic1")
	err := s.UnregisterKnownTopic("topic1")
	if err != nil {
		t.Errorf("UnregisterKnownTopic failed: %v", err)
	}
	for _, topic := range *s.knownTopics {
		if topic == "topic1" {
			t.Error("Topic still exists after UnregisterKnownTopic")
		}
	}
}

func TestJMP_RegisterPubSubWriter(t *testing.T) {
	s := New()
	handler := func(ctx *Context, msg *Message) {
		// Test writer handler: no action needed for registration test
	}
	err := s.RegisterPubSubWriter("writer1", handler)
	if err != nil {
		t.Errorf("RegisterPubSubWriter failed: %v", err)
	}
	err = s.RegisterPubSubWriter("writer1", handler)
	if err == nil {
		t.Error("RegisterPubSubWriter should fail for duplicate registration")
	}
}

func TestJMP_UnregisterPubSubHandler(t *testing.T) {
	s := New()
	s.RegisterPubSubWriter("writer1", func(ctx *Context, msg *Message) {
		// Test writer handler: no action needed for registration test
	})
	s.UnregisterPubSubHandler("writer1")
	if _, ok := s.pubSubWriters["writer1"]; ok {
		t.Error("PubSubWriter still exists after UnregisterPubSubHandler")
	}
}

func TestJMP_HandleMessage_Rpc(t *testing.T) {
	s := New()
	s.RegisterRpc("hello", func(ctx *Context, req *RpcRequestPayload) (res Result) {
		return NewOkResult("string", fmt.Sprintf("hello %s", ctx.GetString("user")))
	})

	msg := &Message{
		Version:  Version,
		Proto:    ProtoRpc,
		Content:  RpcCall,
		Payload:  map[string]any{"method": "hello"},
		Sequence: 2,
	}

	ctx := NewContext()
	ctx.Data["user"] = "Junie"

	resp, err := s.HandleMessage(ctx, msg)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	if resp.Content != RpcResult {
		t.Errorf("Expected %s, got %s", RpcResult, resp.Content)
	}

	payload := resp.Payload.(RpcResponsePayload)
	if payload.Result.Data != "hello Junie" {
		t.Errorf("Expected 'hello Junie', got %v", payload.Result.Data)
	}

	if resp.Sequence != msg.Sequence {
		t.Fatal("Sequence number does not match")
	}
}

func TestJMP_HandleMessage_PubSub(t *testing.T) {
	s := New()
	s.RegisterKnownTopic("news")

	msg := &Message{
		Version: Version,
		Proto:   ProtoPubSub,
		Content: PubSubscribe,
		Payload: map[string]any{"topic": "news"},
	}

	ctx := NewContext()
	resp, err := s.HandleMessage(ctx, msg)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	payload := resp.Payload.(PubSubResponsePayload)
	res := payload.Result.Data.(PubSubSubscriptionResult)
	if !res.Subscribed {
		t.Error("Expected Subscribed to be true")
	}
}

func TestJMP_PubSubAnnounce(t *testing.T) {
	s := New()
	s.RegisterKnownTopic("news")

	received := false
	s.RegisterPubSubWriter("test", func(ctx *Context, msg *Message) {
		received = true
	})

	ctx := NewContext()
	subscribeMsg := &Message{
		Version: Version,
		Proto:   ProtoPubSub,
		Content: PubSubscribe,
		Payload: map[string]any{"topic": "news"},
	}
	s.HandleMessage(ctx, subscribeMsg)

	err := s.PubSubAnnounce("news", nil, "Breaking News", "string")
	if err != nil {
		t.Errorf("PubSubAnnounce failed: %v", err)
	}

	if !received {
		t.Error("PubSubWriter did not receive announcement")
	}
}

func TestJMP_PubSubAnnounceWithMatcher(t *testing.T) {
	s := New()
	s.RegisterKnownTopic("news")

	receivedCount := 0
	s.RegisterPubSubWriter("test", func(ctx *Context, msg *Message) {
		receivedCount++
	})

	ctx1 := NewContext()
	ctx2 := NewContext()

	subscribeMsg := &Message{
		Version: Version,
		Proto:   ProtoPubSub,
		Content: PubSubscribe,
		Payload: map[string]any{"topic": "news"},
	}

	s.HandleMessage(ctx1, subscribeMsg)
	s.HandleMessage(ctx2, subscribeMsg)

	// Matcher that only selects ctx1
	matcher := func(ctx *Context) int64 {
		if ctx.UniqueRequestorId == ctx1.UniqueRequestorId {
			return ctx.UniqueRequestorId
		}
		return 0
	}

	err := s.PubSubAnnounceWithMatcher("news", "Targeted News", "string", matcher)
	if err != nil {
		t.Errorf("PubSubAnnounceWithMatcher failed: %v", err)
	}

	if receivedCount != 1 {
		t.Errorf("Expected 1 announcement, got %d", receivedCount)
	}
}
