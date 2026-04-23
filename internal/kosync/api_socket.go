//
// File:        internal/kosync/api_socket.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"encoding/binary"
	"slices"
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
	_ = app.Jmp.RegisterRpc("documents.all", func(ctx *jmp.Context, rpc *jmp.RpcRequestPayload) jmp.Result {
		result, e := app.apiGetUserDocuments(ctx.GetString(CtxContextUserName))
		if e != nil {
			return jmp.NewErrorResultFromErr(e)
		}
		return jmp.NewOkResult("Array[DocumentWithHistory]", result)
	})

	_ = app.Jmp.RegisterRpc("documents.update", func(ctx *jmp.Context, rpc *jmp.RpcRequestPayload) jmp.Result {
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
			_ = app.PubSubAnnounce(ctx.GetString(CtxContextUserId), PubSubTopicUserDocuments, updatedDoc)
		}()

		return jmp.NewOkResult("Document", updatedDoc)
	})

	_ = app.Jmp.RegisterRpc("disconnect", func(ctx *jmp.Context, rpc *jmp.RpcRequestPayload) jmp.Result {
		ctx.Data[JmpContextDisconnect] = true
		return jmp.NewOkResult(jmp.TypeString, "goodbye.")
	})

	_ = app.Jmp.RegisterKnownTopic("user.documents")
	_ = app.Jmp.RegisterPubSubWriter("RawSocket", func(ctx *jmp.Context, msg *jmp.Message) {
		err := ctx.RawSocket.(*websocket.Conn).WriteJSON(msg)
		if err != nil {
			return
		}
	})
}

func (app *Kosync) HandleWebsocket(c *websocket.Conn) {
	LogDebug("HandleWebsocket")

	token := c.Params("id")
	valid, userId := app.Crypt.VerifyToken(token)
	if !valid {
		err := c.WriteJSON(jmp.NewRpcNoticeMessage(jmp.NewRpcResponseFromResult("connect", jmp.NewErrorResult([]string{
			"Your access token is invalid. Make sure it is supplied like this: /api/ws/{token}",
		}))))

		if err != nil {
			LogDebug("Failed to send WebSocket auth failure message: %v", err.Error())
			return
		}
		return
	}

	user, found, err := app.Db.FindUserById(userId)
	if err != nil {
		LogError("Failed to find user: %v", err.Error())
		return
	}
	if !found {
		LogError("User not found: %s", userId)
		return
	}

	requestId := utils.UUIDv4()
	currClose := c.CloseHandler()
	c.SetCloseHandler(func(code int, text string) error {
		LogDebug("Websocket connection closed: %d %s", code, text)
		*app.WsSubs = slices.DeleteFunc(*app.WsSubs, func(s *WsSub) bool {
			return s.RequestId == requestId
		})
		return currClose(code, text)
	})
	defer func() {
		LogDebug("Websocket connection ended")
		*app.WsSubs = slices.DeleteFunc(*app.WsSubs, func(s *WsSub) bool {
			return s.RequestId == requestId
		})
	}()

	err = c.WriteJSON(jmp.NewRpcNoticeMessage(jmp.NewRpcResponseFromResult("connect", jmp.NewOkResult("ServerInfo", WsInfo{
		ServerName:    "KOsync",
		ServerVersion: Version,
		Message:       "KOsync WebSocket API. Hello!",
	}))))
	if err != nil {
		LogDebug("Failed to send WebSocket welcome message: %v", err.Error())
		return
	}

	ctx := jmp.Context{
		UniqueRequestorId: int64(binary.BigEndian.Uint32([]byte(userId))),
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

func (app *Kosync) PubSubAnnounce(userId string, topic PubSubTopic, data interface{}) error {
	LogDebug("PubSubAnnounce(userId='%s', topic='%s', data=%+v)", userId, topic, data)
	userIdInt64 := int64(binary.BigEndian.Uint32([]byte(userId)))
	err := app.Jmp.PubSubAnnounce(PubSubTopicStrings[topic], &userIdInt64, data, "Document")
	if err != nil {
		return err
	}
	return nil
}
