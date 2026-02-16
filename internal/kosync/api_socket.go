//
// File:        internal/kosync/api_socket.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"fmt"
	"slices"
	"strings"

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

func (app *Kosync) HandleWebsocket(c *websocket.Conn) {
	LogDebug("HandleWebsocket")

	token := c.Params("id")
	valid, userId := app.Crypt.VerifyToken(token)
	if !valid {
		err := c.WriteJSON(WsMessage{
			Type: "rpc",
			Payload: WsResult{
				ForRpc: "connect",
				ErrMsg: "Your access token is invalid. Make sure it is supplied like this: /api/ws/{token}",
			},
		})
		if err != nil {
			LogDebug("Failed to send WebSocket auth failure message: %v", err.Error())
			return
		}
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

	err := c.WriteJSON(WsMessage{
		Type: "rpc",
		Payload: WsInfo{
			ServerName:    "KOsync",
			ServerVersion: Version,
			Message:       "KOsync WebSocket API. Hello!",
		},
	})
	if err != nil {
		LogDebug("Failed to send WebSocket welcome message: %v", err.Error())
		return
	}

	// rpcMethod tries to handle the RPC function; returns true if it was handled
	rpcMethod := func(msg *WsRpc, method string, fun func() error) bool {
		if msg.Method == method {
			err := fun()
			if err != nil {
				LogDebug("Failed to execute RPC method '%s': %v", method, err.Error())
				err2 := c.WriteJSON(newWsError(method, err.Error()))
				if err2 != nil {
					LogDebug("Failed to send WebSocket error message: %v", err.Error())
				}
			}
			return true
		} else {
			LogDebug("RPC method '%s' does not match handler '%s'", msg.Method, method)
			return false
		}
	}

	var msg WsMessage
	for {
		if err := c.ReadJSON(&msg); err != nil {
			LogError("Failed to read WebSocket message: %v", err.Error())
			if strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "going away") {
				return
			}
			continue
		}

		if msg.Type == "rpc" {
			var rpc = WsRpcFromMap(msg.Payload.(map[string]interface{}))
			LogDebug("Received RPC: %s", rpc.Method)
			var handled = false

			handled = rpcMethod(&rpc, "documents.all", func() (e error) {
				result, e := app.apiGetUserDocuments(c.Locals(CtxContextUserName).(string))
				if e != nil {
					return
				}
				e = c.WriteJSON(newWsResult(rpc.Method, *result))
				if e != nil {
					return
				}
				return
			})
			if handled {
				continue
			}

			handled = rpcMethod(&rpc, "documents.update", func() (e error) {
				rpcDoc, found := rpc.Arguments["document"]
				if !found {
					return fmt.Errorf("RPC call is missing the argument 'document'")
				}

				doc := DocumentFromMap(rpcDoc.(map[string]interface{}))

				e = app.Db.CreateOrUpdateDocument(&doc)
				if e != nil {
					return
				}

				updatedDoc, _, e := app.Db.FindDocumentById(userId, doc.Id)
				if e != nil {
					return
				}

				e = c.WriteJSON(newWsResult(rpc.Method, updatedDoc))

				go func() {
					_ = app.PubSubAnnounce(userId, PubSubTopicUserDocuments, updatedDoc)
				}()

				return
			})
			if handled {
				continue
			}

			handled = rpcMethod(&rpc, "disconnect", func() (e error) {
				e = c.WriteJSON(newWsResult(rpc.Method, "goodbye."))
				if e != nil {
					return
				}
				e = c.Close()
				return
			})
			if handled {
				return
			}

			LogDebug("Unknown RPC method: '%s': %+v", rpc.Method, rpc)
			err := c.WriteJSON(newWsError(rpc.Method, fmt.Sprintf("Unknown RPC method: '%s'", rpc.Method)))
			if err != nil {
				LogDebug("Failed to send WebSocket error message: %v", err.Error())
				return
			}
		} else if msg.Type == "pubsub" {
			var rpc = WsPubsubFromMap(msg.Payload.(map[string]interface{}))

			topicId := PubSubTopicFromString(rpc.Topic)
			if topicId == PubSubTopicUnknown {
				LogDebug("Unknown PubSub topic: '%s'", rpc.Topic)
				err := c.WriteJSON(newWsError("pubsub", fmt.Sprintf("Unknown PubSub topic: '%s'", rpc.Topic)))
				if err != nil {
					LogDebug("Failed to send WebSocket error message: %v", err.Error())
					return
				}
			}
			LogDebug("Received PubSub: %s", rpc.Topic)

			alreadyInSubs := slices.ContainsFunc(*app.WsSubs, func(s *WsSub) bool {
				return s.RequestId == requestId && s.Topic == topicId
			})
			if !alreadyInSubs {
				*app.WsSubs = append(*app.WsSubs, &WsSub{
					UserId:    userId,
					RequestId: requestId,
					Topic:     topicId,
					Socket:    c,
				})
			}

			err := c.WriteJSON(newPsResult(topicId, "subscribed"))
			if err != nil {
				LogDebug("Failed to send WebSocket result: %v", err.Error())
				return
			}
		}
	}
}

func (app *Kosync) PubSubAnnounce(userId string, topic PubSubTopic, data interface{}) error {
	LogDebug("PubSubAnnounce(userId='%s', topic='%s', data=%+v)", userId, topic, data)
	for sub := range slices.Values(*app.WsSubs) {
		if sub.UserId == userId && sub.Topic == topic {
			err := sub.Socket.WriteJSON(newPsResult(topic, data))
			if err != nil {
				LogDebug("Failed to send PubSub message: %v", err.Error())
				continue
			}
		}
	}
	return nil
}
