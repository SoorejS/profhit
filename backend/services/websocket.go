package services

import (
	"github.com/gorilla/websocket"
	"log"
	"profhit-backend/middleware"
	"sync"
	"time"
)

type WSMessage struct {
	TargetUserID uint        `json:"-"`
	Event        string      `json:"event"`
	Payload      interface{} `json:"payload"`
}

type Client struct {
	Conn   *websocket.Conn
	UserID uint
	Token  string
	send   chan WSMessage
	done   chan struct{}
}

var clients = make(map[*Client]bool)
var broadcast = make(chan WSMessage, 256)
var clientsMutex sync.Mutex
var hubOnce sync.Once

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = pongWait * 9 / 10
	maxMessageSize = 512
)

func StartWebSocketHub() { hubOnce.Do(func() { go HandleConnections() }) }

// Each connection has one bounded writer. A slow client cannot block other
// users or create unbounded goroutines. The reader owns disconnect cleanup.
func UpgradeAndRegister(conn *websocket.Conn, userID uint, token string) {
	client := &Client{Conn: conn, UserID: userID, Token: token, send: make(chan WSMessage, 32), done: make(chan struct{})}
	clientsMutex.Lock()
	clients[client] = true
	clientsMutex.Unlock()
	defer func() {
		clientsMutex.Lock()
		delete(clients, client)
		clientsMutex.Unlock()
		close(client.done)
		conn.Close()
	}()
	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().UTC().Add(pongWait))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().UTC().Add(pongWait)) })
	client.send <- WSMessage{Event: "connected", Payload: "Authenticated connection established"}
	go client.writeLoop()
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (client *Client) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	defer client.Conn.Close()
	for {
		var msg WSMessage
		ping := false
		select {
		case <-client.done:
			return
		case <-ticker.C:
			ping = true
		case msg = <-client.send:
		}
		// Handshake authentication is not sufficient for long-lived connections.
		if _, _, err := middleware.ValidateToken(client.Token); err != nil {
			return
		}
		client.Conn.SetWriteDeadline(time.Now().UTC().Add(writeWait))
		var err error
		if ping {
			err = client.Conn.WriteMessage(websocket.PingMessage, nil)
		} else {
			err = client.Conn.WriteJSON(msg)
		}
		if err != nil {
			return
		}
	}
}

func HandleConnections() {
	for msg := range broadcast {
		clientsMutex.Lock()
		for client := range clients {
			if msg.TargetUserID != 0 && client.UserID != msg.TargetUserID {
				continue
			}
			select {
			case <-client.done:
			case client.send <- msg:
			default:
				client.Conn.Close() // Consumer is too slow; reconnect and refetch.
			}
		}
		clientsMutex.Unlock()
	}
}

func BroadcastToAll(event string, payload interface{}) {
	enqueue(WSMessage{Event: event, Payload: payload})
}
func BroadcastToUser(userID uint, event string, payload interface{}) {
	if userID == 0 {
		log.Println("[ws] refused private event without recipient")
		return
	}
	enqueue(WSMessage{TargetUserID: userID, Event: event, Payload: payload})
}
func enqueue(msg WSMessage) {
	select {
	case broadcast <- msg:
	default:
		log.Println("[ws] event queue full; clients must refetch state")
	}
}
