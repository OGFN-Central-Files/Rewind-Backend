package XMPP

import (
	"encoding/json"
	"go-backend/src/global"
	"log"

	"github.com/gorilla/websocket"
)

type XmppService struct{}

func (f *XmppService) SendXmppMessageToClient(accountId string, body interface{}) {
	if !global.XmppServerConnected {
		return // make sure XMPP is connected so it won't crash ts
	}

	WebSocketConn := global.WebSocketConn
	bodyJson, err := json.Marshal(body)
	if err != nil {
		log.Printf("Error marshaling body to JSON: %v", err)
		return
	}

	payload := map[string]interface{}{
		"payload":   "SendXmppMessageToClient",
		"accountId": accountId,
		"body":      bodyJson,
	}

	payloadJson, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling payload to JSON: %v", err)
		return
	}

	if err := WebSocketConn.WriteMessage(websocket.TextMessage, payloadJson); err != nil {
		log.Printf("Error sending message to WebSocket: %v", err)
		return
	}
}

func (f *XmppService) getPresenceFromUser(fromId string, toId string, offline bool) {

}

func (f *XmppService) SendGetPlayerPayload() {
	if !global.XmppServerConnected {
		return // make sure XMPP is connected so it won't crash ts
	}

	WebSocketConn := global.WebSocketConn

	payload := map[string]interface{}{
		"payload": "FetchPlayerCountForAdmin",
	}

	payloadJson, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling payload to JSON: %v", err)
		return
	}

	if err := WebSocketConn.WriteMessage(websocket.TextMessage, payloadJson); err != nil {
		log.Printf("Error sending message to WebSocket: %v", err)
		return
	}
}
