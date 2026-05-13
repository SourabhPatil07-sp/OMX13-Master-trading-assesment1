package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for MVP
	},
}

// Hub maintains the set of active clients and broadcasts messages to them.
type Hub struct {
	// Registered clients. Map of marketID to clients
	clients map[int]map[*Client]bool

	// Inbound messages from the clients.
	Broadcast chan Message

	// Register requests from the clients.
	Register chan *Client

	// Unregister requests from clients.
	Unregister chan *Client

	mu sync.RWMutex
}

type Message struct {
	MarketID int         `json:"market_id"`
	Type     string      `json:"type"` // e.g., "TRADE", "ORDERBOOK", "AMM_PRICE"
	Data     interface{} `json:"data"`
}

func NewHub() *Hub {
	return &Hub{
		Broadcast:  make(chan Message),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		clients:    make(map[int]map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			if _, ok := h.clients[client.MarketID]; !ok {
				h.clients[client.MarketID] = make(map[*Client]bool)
			}
			h.clients[client.MarketID][client] = true
			h.mu.Unlock()
			log.Printf("Client connected to market %d", client.MarketID)

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.MarketID][client]; ok {
				delete(h.clients[client.MarketID], client)
				close(client.send)
				log.Printf("Client disconnected from market %d", client.MarketID)
			}
			h.mu.Unlock()

		case message := <-h.Broadcast:
			h.mu.RLock()
			for client := range h.clients[message.MarketID] {
				msgBytes, err := json.Marshal(message)
				if err != nil {
					continue
				}
				select {
				case client.send <- msgBytes:
				default:
					close(client.send)
					delete(h.clients[message.MarketID], client)
				}
			}
			h.mu.RUnlock()
		}
	}
}
