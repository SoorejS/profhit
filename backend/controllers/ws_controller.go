package controllers

import (
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// In dev (APP_URL empty), allow all. In prod, check origin.
		appURL := os.Getenv("APP_URL")
		if appURL == "" {
			return true
		}
		origin := r.Header.Get("Origin")
		return origin == appURL || origin == appURL+"/"
	},
}


var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan interface{})
var mutex = &sync.Mutex{}

func WsHandler(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ws] upgrade error: %v", err)
		return
	}
	defer ws.Close()

	mutex.Lock()
	clients[ws] = true
	mutex.Unlock()

	for {
		var msg interface{}
		err := ws.ReadJSON(&msg)
		if err != nil {
			mutex.Lock()
			delete(clients, ws)
			mutex.Unlock()
			break
		}
	}
}

func HandleMessages() {
	for {
		msg := <-broadcast
		mutex.Lock()
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				client.Close()
				delete(clients, client)
			}
		}
		mutex.Unlock()
	}
}

func BroadcastUpdate(event string, payload interface{}) {
	msg := gin.H{
		"event":   event,
		"payload": payload,
	}
	broadcast <- msg
}
