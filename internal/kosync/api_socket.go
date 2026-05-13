//
// File:        internal/kosync/api_socket.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"encoding/binary"
	"strings"

	"git.obth.eu/atjontv/kosync/pkg/jmp"
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

func (app *Kosync) HandleOpenWebsocket(c fiber.Ctx) error {
	LogDebug("HandleOpenWebsocket")
	redirUrl := strings.Replace(c.BaseURL(), "http:", "ws:", 1) + "/api/ws/"
	if c.Locals(CtxContextUserId) != nil && c.Locals(CtxContextUserName) != nil {
		token, err := app.Crypt.CreateToken(c.Locals(CtxContextUserId).(string), c.Locals(CtxContextUserName).(string))
		if err != nil {
			return err
		}
		redirUrl = redirUrl + token
	}
	if websocket.IsWebSocketUpgrade(c) {
		return c.Redirect().To(redirUrl)
	}
	c.Set(fiber.HeaderConnection, fiber.HeaderUpgrade)
	c.Set(fiber.HeaderUpgrade, "websocket")
	return c.Redirect().Status(fiber.StatusSeeOther).To(redirUrl)
}

const JmpContextRequestId = "request_id"
const JmpContextDisconnect = "disconnect"

func (app *Kosync) ConfigureJmp() {
	_ = app.Jmp.RegisterRpc("documents.all", app.RpcDocumentsAll)
	_ = app.Jmp.RegisterRpc("documents.update", app.RpcDocumentsUpdate)
	_ = app.Jmp.RegisterRpc("documents.delete", app.RpcDocumentsDelete)
	_ = app.Jmp.RegisterRpc("documents.history.delete", app.RpcDocumentsHistoryDelete)
	_ = app.Jmp.RegisterRpc("documents.history.restore", app.RpcDocumentsHistoryRestore)

	_ = app.Jmp.RegisterRpc("disconnect", app.RpcDisconnect)

	_ = app.Jmp.RegisterKnownTopic("user.documents")
	_ = app.Jmp.RegisterPubSubWriter("RawSocket", func(ctx *jmp.Context, msg *jmp.Message) {
		soc := ctx.RawSocket.(*websocket.Conn)
		LogDebug("Sending pubsub message to client (IP '%s') for Request '%d'", soc.IP(), ctx.UniqueRequestorId)
		err := soc.WriteJSON(msg)
		if err != nil {
			LogError("Failed to send message to raw socket: %v", err.Error())
			return
		}
	})
}

func (app *Kosync) authenticateWebsocket(c *websocket.Conn) (string, *User, bool) {
	token := c.Params("id")
	valid, userId := app.Crypt.VerifyToken(token)
	if !valid {
		err := c.WriteJSON(jmp.NewRpcNoticeMessage(jmp.NewRpcResponseFromResult("connect", jmp.NewErrorResult([]string{
			"Your access token is invalid. Make sure it is supplied like this: /api/ws/{token}",
		}))))

		if err != nil {
			LogDebug("Failed to send WebSocket auth failure message: %v", err.Error())
		}
		return "", nil, false
	}

	user, found, err := app.Db.FindUserById(userId)
	if err != nil {
		LogError("Failed to find user: %v", err.Error())
		return "", nil, false
	}
	if !found {
		LogError("User not found: %s", userId)
		return "", nil, false
	}
	return userId, user, true
}

func (app *Kosync) HandleWebsocket(c *websocket.Conn) {
	LogDebug("HandleWebsocket")

	if strings.ToLower(c.Subprotocol()) != "jmp" {
		_ = c.WriteMessage(websocket.TextMessage, []byte("You do not seem to support JMP (JSON Message Protocol). Please use a WebSocket Client with JMP support: https://git.obth.eu/atjontv/kosync/-/blob/main/pkg/jmp/README.md"))
		_ = c.Close()
		return
	}

	userId, user, ok := app.authenticateWebsocket(c)
	if !ok {
		return
	}

	requestId := utils.UUIDv4()
	uniqueReqId := int64(binary.BigEndian.Uint32([]byte(requestId)))

	currClose := c.CloseHandler()
	c.SetCloseHandler(func(code int, text string) error {
		app.Jmp.InvalidatePubSubSubscriptionForRequestId(uniqueReqId)
		LogDebug("Websocket connection closed: %d %s", code, text)
		return currClose(code, text)
	})
	defer func() {
		app.Jmp.InvalidatePubSubSubscriptionForRequestId(uniqueReqId)
		LogDebug("Websocket connection ended")
	}()

	err := c.WriteJSON(jmp.NewRpcNoticeMessage(jmp.NewRpcResponseFromResult("connect", jmp.NewOkResult("ServerInfo", WsInfo{
		ServerName:    "KOsync",
		ServerVersion: Version,
		Message:       "KOsync WebSocket API. Hello!",
	}))))
	if err != nil {
		LogDebug("Failed to send WebSocket welcome message: %v", err.Error())
		return
	}

	ctx := jmp.Context{
		UniqueRequestorId: uniqueReqId,
		Data: map[string]interface{}{
			JmpContextRequestId: requestId,
			CtxContextUserId:    userId,
			CtxContextUserName:  user.Username,
		},
		RawSocket: c,
	}

	//var msg WsMessage
	var msg map[string]any
	for {
		if err := c.ReadJSON(&msg); err != nil {
			LogError("Failed to read WebSocket message: %v", err.Error())
			if strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "going away") {
				return
			}
			continue
		}

		jmpMsg, err := jmp.MessageFromMap(msg)
		if err != nil {
			LogError("Failed to parse WebSocket message: %v", err.Error())
			continue
		}

		res, err := app.Jmp.HandleMessage(&ctx, jmpMsg)
		if err != nil {
			LogError("Failed to handle RPC: %v", err.Error())
			err = c.WriteJSON(res)
			if err != nil {
				LogError("Failed to send WebSocket response: %v", err.Error())
			}
			continue
		}

		err = c.WriteJSON(res)
		if err != nil {
			LogError("Failed to send WebSocket response: %v", err.Error())
		}

		if ctx.Data[JmpContextDisconnect] == true {
			err = c.Close()
			if err != nil {
				LogError("Failed to close WebSocket connection: %v", err.Error())
			}
			return
		}
	}
}

func (app *Kosync) PubSubAnnounce(userId string, topic PubSubTopic, data interface{}, typeHint string) error {
	LogDebug("PubSubAnnounce(userId='%s', topic='%s', data=%+v)", userId, topic, data)
	err := app.Jmp.PubSubAnnounceWithMatcher(PubSubTopicStrings[topic], data, typeHint, func(ctx *jmp.Context) int64 {
		if ctx.GetString(CtxContextUserId) == userId {
			return ctx.UniqueRequestorId
		}
		return 0
	})
	if err != nil {
		return err
	}
	return nil
}

func (app *Kosync) RpcDocumentsAll(ctx *jmp.Context, rpc *jmp.RpcRequestPayload) jmp.Result {
	result, e := app.apiGetUserDocuments(ctx.GetString(CtxContextUserName))
	if e != nil {
		return jmp.NewErrorResultFromErr(e)
	}
	return jmp.NewOkResult("Array[DocumentWithHistory]", result)
}

func (app *Kosync) RpcDocumentsUpdate(ctx *jmp.Context, rpc *jmp.RpcRequestPayload) jmp.Result {
	rpcDoc, found := rpc.Arguments["document"]
	if !found {
		return jmp.NewErrorResult([]string{"RPC call is missing the argument 'document'"})
	}

	doc := DocumentFromMap(rpcDoc.(map[string]interface{}))

	err := app.Db.CreateOrUpdateDocument(&doc)
	if err != nil {
		return jmp.NewErrorResultFromErr(err)
	}

	updatedDoc, _, e := app.Db.FindDocumentById(ctx.GetString(CtxContextUserId), doc.Id)
	if e != nil {
		return jmp.NewErrorResultFromErr(e)
	}

	go func() {
		_ = app.PubSubAnnounce(ctx.GetString(CtxContextUserId), PubSubTopicUserDocuments, updatedDoc, "Document")
	}()

	return jmp.NewOkResult("Document", updatedDoc)
}

func (app *Kosync) RpcDocumentsDelete(ctx *jmp.Context, rpc *jmp.RpcRequestPayload) jmp.Result {
	rpcDocId, err := getRpcArgumentString(rpc.Arguments, "document_id")
	if err != nil {
		return jmp.NewErrorResult([]string{err.Error()})
	}

	err = app.Db.DeleteDocumentById(ctx.GetString(CtxContextUserId), rpcDocId)
	if err != nil {
		return jmp.NewErrorResultFromErr(err)
	}

	go func(userId, documentId string) {
		type DocumentDeletion struct {
			DocumentId string `json:"document_id"`
		}
		_ = app.PubSubAnnounce(userId, PubSubTopicUserDocuments, DocumentDeletion{DocumentId: documentId}, "DocumentDeletion")
	}(ctx.GetString(CtxContextUserId), rpcDocId)

	return jmp.NewOkResult(jmp.TypeString, "ok")
}

func (app *Kosync) RpcDocumentsHistoryDelete(ctx *jmp.Context, payload *jmp.RpcRequestPayload) jmp.Result {
	rpcDocId, err := getRpcArgumentString(payload.Arguments, "document_id")
	if err != nil {
		return jmp.NewErrorResult([]string{err.Error()})
	}

	lastReadAt, err := getRpcArgumentInt64(payload.Arguments, "last_read_at")
	if err != nil {
		return jmp.NewErrorResult([]string{err.Error()})
	}

	err = app.Db.DeleteDocumentHistoryItem(ctx.GetString(CtxContextUserId), rpcDocId, lastReadAt)
	if err != nil {
		return jmp.NewErrorResultFromErr(err)
	}

	go func(userId, documentId string, lastReadAt int64) {
		type HistoryDeletion struct {
			DocumentId string `json:"document_id"`
			LastReadAt int64  `json:"last_read_at"`
		}
		_ = app.PubSubAnnounce(userId, PubSubTopicUserDocuments, HistoryDeletion{DocumentId: documentId, LastReadAt: lastReadAt}, "HistoryDeletion")
	}(ctx.GetString(CtxContextUserId), rpcDocId, lastReadAt)

	return jmp.NewOkResult(jmp.TypeString, "ok")
}

func (app *Kosync) RpcDocumentsHistoryRestore(ctx *jmp.Context, payload *jmp.RpcRequestPayload) jmp.Result {
	rpcDocId, err := getRpcArgumentString(payload.Arguments, "document_id")
	if err != nil {
		return jmp.NewErrorResult([]string{err.Error()})
	}

	lastReadAt, err := getRpcArgumentInt64(payload.Arguments, "last_read_at")
	if err != nil {
		return jmp.NewErrorResult([]string{err.Error()})
	}

	err = app.Db.RestoreDocumentHistoryItem(ctx.GetString(CtxContextUserId), rpcDocId, lastReadAt)
	if err != nil {
		return jmp.NewErrorResultFromErr(err)
	}

	go func(userId, documentId string, lastReadAt int64) {
		type HistoryRestore struct {
			DocumentId string `json:"document_id"`
			LastReadAt int64  `json:"last_read_at"`
		}
		_ = app.PubSubAnnounce(userId, PubSubTopicUserDocuments, HistoryRestore{DocumentId: documentId, LastReadAt: lastReadAt}, "HistoryRestore")
	}(ctx.GetString(CtxContextUserId), rpcDocId, lastReadAt)

	return jmp.NewOkResult(jmp.TypeString, "ok")
}

func (app *Kosync) RpcDisconnect(ctx *jmp.Context, _ *jmp.RpcRequestPayload) jmp.Result {
	ctx.Data[JmpContextDisconnect] = true
	return jmp.NewOkResult(jmp.TypeString, "goodbye.")
}
