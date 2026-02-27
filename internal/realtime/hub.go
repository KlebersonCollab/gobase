package realtime

import (
	"encoding/json"
	"log"
)

type EventType string

const (
	EventInsert EventType = "INSERT"
	EventUpdate EventType = "UPDATE"
	EventDelete EventType = "DELETE"
)

type BroadcastMessage struct {
	TenantID string
	Table    string
	Action   EventType
	Data     interface{}
}

type OutputMessage struct {
	Table  string      `json:"table"`
	Action EventType   `json:"action"`
	Data   interface{} `json:"data"`
}

type Hub struct {
	Clients    map[*Client]bool
	Broadcast  chan *BroadcastMessage
	Register   chan *Client
	Unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		Broadcast:  make(chan *BroadcastMessage),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Clients:    make(map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Clients[client] = true
		case client := <-h.Unregister:
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.send)
			}
		case message := <-h.Broadcast:
			log.Printf("[HUB] Broadcasting to tenant %s: %s on %s\n", message.TenantID, message.Action, message.Table)
			// Package the message for the client
			out := OutputMessage{
				Table:  message.Table,
				Action: message.Action,
				Data:   message.Data,
			}
			outBytes, err := json.Marshal(out)
			if err != nil {
				log.Printf("Error marshaling broadcast: %v", err)
				continue
			}

			count := 0
			for client := range h.Clients {
				// Only send to clients of the SAME tenant
				if client.TenantID == message.TenantID {
					select {
					case client.send <- outBytes:
						count++
					default:
						// If the client's channel is full/blocked, disconnect them
						close(client.send)
						delete(h.Clients, client)
					}
				}
			}
			log.Printf("[HUB] Delivered to %d clients\n", count)
		}
	}
}
