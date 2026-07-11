package services

import (
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WSMessage struct {
	TargetUserID uint        `json:"-"` // 0 means broadcast to all
	Event        string      `json:"event"`
	Payload      interface{} `json:"payload"`
}

type Client struct {
	Conn   *websocket.Conn
	UserID uint
	Mu     sync.Mutex
}

var clients = make(map[*Client]bool)
var broadcast = make(chan WSMessage, 256)
var clientsMutex = &sync.Mutex{}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// UpgradeAndRegister registers a new websocket connection
func UpgradeAndRegister(conn *websocket.Conn, userID uint) {
	client := &Client{
		Conn:   conn,
		UserID: userID,
	}

	clientsMutex.Lock()
	clients[client] = true
	clientsMutex.Unlock()

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error { conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	// Start ping ticker for this client
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer func() {
			ticker.Stop()
			conn.Close()
		}()
		for range ticker.C {
			client.Mu.Lock()
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				client.Mu.Unlock()
				return
			}
			client.Mu.Unlock()
		}
	}()

	// Read loop
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			clientsMutex.Lock()
			delete(clients, client)
			clientsMutex.Unlock()
			conn.Close()
			break
		}
	}
}

// HandleConnections processes messages from the broadcast channel
func HandleConnections() {
	for {
		msg := <-broadcast
		clientsMutex.Lock()
		for client := range clients {
			// If TargetUserID is > 0, only send to that specific user
			if msg.TargetUserID > 0 && client.UserID != msg.TargetUserID {
				continue
			}

			client.Mu.Lock()
			client.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := client.Conn.WriteJSON(gin.H{
				"event":   msg.Event,
				"payload": msg.Payload,
			})
			if err != nil {
				client.Conn.Close()
				delete(clients, client)
			}
			client.Mu.Unlock()
		}
		clientsMutex.Unlock()
	}
}

// BroadcastToAll sends a message to all connected clients
func BroadcastToAll(event string, payload interface{}) {
	msg := WSMessage{
		TargetUserID: 0,
		Event:        event,
		Payload:      payload,
	}
	select {
	case broadcast <- msg:
	default:
		log.Println("[ws] broadcast channel full or no receiver, dropping message")
	}
}

// BroadcastToUser sends a message to a specific user
func BroadcastToUser(userID uint, event string, payload interface{}) {
	if userID == 0 {
		log.Println("[ws] cannot broadcast to user id 0")
		return
	}
	msg := WSMessage{
		TargetUserID: userID,
		Event:        event,
		Payload:      payload,
	}
	select {
	case broadcast <- msg:
	default:
		log.Println("[ws] broadcast channel full or no receiver, dropping message")
	}
}
