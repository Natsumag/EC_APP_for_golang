package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
)

type WebSocketConnection struct {
	*websocket.Conn
}

type WebSocketPayload struct {
	Action      string              `json:"action"`
	Message     string              `json:"message"`
	UserID      int                 `json:"user_id"`
	UserName    string              `json:"user_name"`
	MessageType string              `json:"message_type"`
	Connection  WebSocketConnection `json:"-"`
}

type WebSocketJsonResponse struct {
	Action  string `json:"action"`
	Message string `json:"message"`
	UserID  int    `json:"user_id"`
}

var upgradeConnection = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

var clients = make(map[WebSocketConnection]string)

var websocketChannel = make(chan WebSocketPayload)

func (app *application) WebsocketEndPoint(w http.ResponseWriter, r *http.Request) {
	ws, err := upgradeConnection.Upgrade(w, r, nil)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	app.infoLog.Println(fmt.Sprintf("Client connected from %s", r.RemoteAddr))
	var response WebSocketJsonResponse
	response.Message = "connected to server"

	err = ws.WriteJSON(response)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	connection := WebSocketConnection{Conn: ws}
	clients[connection] = ""

	go app.ListenForWebSocket(&connection)
}

func (app *application) ListenForWebSocket(connection *WebSocketConnection) {
	defer func() {
		// エラーがあった場合にクライアントを削除
		delete(clients, *connection)
		connection.Close()
	}()

	defer func() {
		if r := recover(); r != nil {
			app.errorLog.Println("ERORR:", fmt.Sprintf("%v", r))
		}
	}()

	var payload WebSocketPayload
	for {
		err := connection.ReadJSON(&payload)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				app.errorLog.Printf("Error: %v", err)
			}
			break
		} else {
			payload.Connection = *connection
			websocketChannel <- payload
		}
	}
}

func (app *application) ListenWebSocketChannel() {
	var response WebSocketJsonResponse
	for {
		e := <-websocketChannel
		switch e.Action {
		case "deleteUser":
			response.Action = "logout"
			response.Message = "your account has been deleted"
			response.UserID = e.UserID
			app.broadcastToAll(response)
		default:
		}
	}
}

func (app *application) broadcastToAll(response WebSocketJsonResponse) {
	for client := range clients {
		err := client.WriteJSON(response)
		if err != nil {
			app.errorLog.Printf("websocket err on %s: %s", response.Action, err)
			_ = client.Close()
			delete(clients, client)
		}
	}
}
