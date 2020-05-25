package main

import (
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

type socketClient struct {
	hub      *hub
	conn     *websocket.Conn
	location string
	send     chan []byte
}

func (sc *socketClient) reader() {
	defer func() {
		sc.conn.Close()
		sc.hub.unregister <- sc
	}()

	for {
		_, message, err := sc.conn.ReadMessage()

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("closed yo: %v", err)
			}
			return
		}

		// the only thing they should be writing is their location
		sc.location = string(message)
	}
}

func (sc *socketClient) writer() {
	for {
		select {
		case message, ok := <-sc.send:
			if !ok {
				return
			}

			w, err := sc.conn.NextWriter(websocket.TextMessage)

			if err != nil {
				log.Println(err)
				return
			}

			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func serveWs(h *hub, writer http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(writer, r, nil)

	if err != nil {
		log.Println(err.Error())
	}

	client := socketClient{
		conn:     conn,
		location: "",
		send:     make(chan []byte, 256),
		hub:      h,
	}

	h.register <- &client

	go client.reader()
	go client.writer()
}
