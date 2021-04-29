package pkg

import (
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
)

type RelayHandler struct {
	mutex   *sync.RWMutex
	clients map[string]*websocket.Conn
}

func (rh *RelayHandler) putClient(id string, con *websocket.Conn) {
	rh.mutex.Lock()
	rh.clients[id] = con
	rh.mutex.Unlock()
}

func (rh *RelayHandler) deleteClient(id string) {
	rh.mutex.Lock()
	delete(rh.clients, id)
	rh.mutex.Unlock()
}

func (rh *RelayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	client, err := Server.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	if r.Header.Get("authorization") == "" || r.Header.Get("authorization") != GetAuth() {
		_ = client.Close()
		return
	}
	id := RandomID()
	if id == "" {
		return
	}
	rh.putClient(id, client)
	go func() {
		for {
			packet := Packet{}
			err := client.ReadJSON(&packet)
			if err != nil {
				rh.deleteClient(id)
				return
			}
			// Type 0 is dispatch
			// Type 1 is receive
			if packet.Type == 0 {
				rh.mutex.RLock()
				for _, c := range rh.clients {
					_ = c.WriteJSON(Packet{
						Type: 1,
						Body: packet.Body,
					})
				}
				rh.mutex.RUnlock()
			}
		}
	}()
}
