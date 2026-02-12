//
// File:        internal/kosync/websocket.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import "github.com/gofiber/contrib/v3/websocket"

type WsSub struct {
	UserId    string
	RequestId string
	Topic     string
	Socket    *websocket.Conn
}

type WsMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type WsRpc struct {
	Method    string                 `json:"method"`
	Arguments map[string]interface{} `json:"arguments"`
}

func WsRpcFromMap(m map[string]interface{}) WsRpc {
	var arguments map[string]interface{} = nil
	args, f := m["arguments"]
	if f {
		arguments = args.(map[string]interface{})
	}
	return WsRpc{
		Method:    m["method"].(string),
		Arguments: arguments,
	}
}

type WsPubsub struct {
	Topic string `json:"topic"`
}

func WsPubsubFromMap(m map[string]interface{}) WsPubsub {
	return WsPubsub{Topic: m["topic"].(string)}
}

type WsResult struct {
	ForRpc string      `json:"for_rpc"`
	Result interface{} `json:"result"`
	ErrMsg string      `json:"error"`
}

type WsAnnounce struct {
	ForTopic string      `json:"for_topic"`
	Data     interface{} `json:"data"`
}

type WsInfo struct {
	ServerName    string `json:"server_name"`
	ServerVersion string `json:"server_version"`
	Message       string `json:"message"`
}

func newWsResult(typ string, result interface{}) WsMessage {
	return WsMessage{
		Type:    "rpc",
		Payload: WsResult{ForRpc: typ, Result: result},
	}
}

func newWsError(typ, message string) WsMessage {
	return WsMessage{
		Type:    "rpc",
		Payload: WsResult{ForRpc: typ, ErrMsg: message},
	}
}

func newPsResult(topic string, result interface{}) WsMessage {
	return WsMessage{
		Type:    "pubsub",
		Payload: WsAnnounce{ForTopic: topic, Data: result},
	}
}
