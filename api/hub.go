package main

import "log"

type hub struct {
	clients map[*socketClient]bool

	register   chan *socketClient
	unregister chan *socketClient
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
		}
	}
}
