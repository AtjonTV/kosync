//
// File:        pkg/jmp/jmp_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package jmp_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"git.obth.eu/atjontv/kosync/pkg/jmp"
)

// TODO: Convert into actual tests
func TestNew(t *testing.T) {
	var s = jmp.New()
	err := s.RegisterRpc("hello", func(ctx *jmp.Context, req *jmp.RpcRequestPayload) (res jmp.Result) {
		return jmp.Result{TypeHint: "string", Data: fmt.Sprintf("meow to you '%s'", ctx.Data["username"])}
	})
	if err != nil {
		panic(err)
	}

	var data map[string]any
	err = json.Unmarshal([]byte(`{"jmp": "1", "proto":"rpc","content":"rpc.call","payload":{"method":"hello"}}`), &data)
	if err != nil {
		panic(err)
	}
	//msg := &data
	msg, err := jmp.MessageFromMap(data)
	if err != nil {
		panic(err)
	}

	ctx := jmp.Context{
		UniqueRequestorId: 1,
		Data:              map[string]any{"username": "USER"},
	}

	t.Logf("Snd: %+v\n", *msg)
	r, err := s.HandleMessage(&ctx, msg)
	if err != nil {
		panic(err)
	}

	t.Logf("Rcv: %+v\n", *r)
	fmt.Println("---------------------")

	_ = s.RegisterKnownTopic("meow")
	_ = s.RegisterPubSubWriter("someName", func(ctx *jmp.Context, msg *jmp.Message) {
		t.Logf("Sub: %+v\n", *msg)
	})

	subscribe := &jmp.Message{
		Version: jmp.Version,
		Proto:   jmp.ProtoPubSub,
		Content: jmp.PubSubscribe,
		Payload: map[string]any{"topic": "meow"},
	}
	t.Logf("Snd: %+v\n", *subscribe)
	r, err = s.HandleMessage(&ctx, subscribe)
	if err != nil {
		panic(err)
	}
	t.Logf("Rcv: %+v\n", *r)

	rep := int64(2)
	_ = s.PubSubAnnounce("meow", &rep, "Sound of a cat", "string")
}
