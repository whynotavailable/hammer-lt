package main

import (
	"encoding/json"
	"hammer-api/shared"
	"log"
)

type hub struct {
	clients map[*socketClient]bool

	register   chan *socketClient
	unregister chan *socketClient

	send chan shared.SocketResponse
}

func (h *hub) Runner() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if h.clients[client] {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.send:
			for client := range h.clients {
				if client.location == message.Type {
					data, err := json.Marshal(message)

					if err != nil {
						log.Println(err.Error())
						continue
					}

					client.send <- data
				}
			}
		}
	}
}
