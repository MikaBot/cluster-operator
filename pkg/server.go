package pkg

import (
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"net/http"
	"sync"
)

var (
	Server *WSServer
	lock   = &sync.Mutex{}
)

const (
	Handshaking      = iota // client -> server
	ShardData               // server -> client
	Ping                    // server -> client
	PingAck                 // client -> server
	Eval                    // client -> server
	BroadcastEval           // client -> server
	BroadcastEvalAck        // server -> client
	Stats                   // server -> client
	StatsAck                // client -> server
	// Misc events
	Ready // client -> server
)

type WSServer struct {
	Clients   []*Cluster
	Upgrader  websocket.Upgrader
	ChanMutex *sync.RWMutex
	Channels  map[string]chan EvalRes
}

type SocketHandler struct{}

type Packet struct {
	Type int         `json:"type"`
	Body interface{} `json:"body,omitempty"`
}

func (w *WSServer) PutEvalChan(id string) {
	w.ChanMutex.Lock()
	w.Channels[id] = make(chan EvalRes)
	w.ChanMutex.Unlock()
}

func (w *WSServer) GetEvalChan(id string) chan EvalRes {
	w.ChanMutex.RLock()
	val, ok := w.Channels[id]
	if ok {
		w.ChanMutex.RUnlock()
		return val
	}
	w.ChanMutex.RUnlock()
	return nil
}

func (w *WSServer) DeleteEvalChan(id string) {
	w.ChanMutex.Lock()
	delete(w.Channels, id)
	w.ChanMutex.Unlock()
}

func (*SocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	client, err := Server.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	if r.Header.Get("authorization") == "" || r.Header.Get("authorization") != GetAuth() {
		_ = client.Close()
		return
	}
	id := NextClusterID()
	if id == -1 {
		_ = client.Close()
		return
	}
	c := Server.Clients[id]
	c.Client = client
	c.State = ClusterConnecting
	go func() {
		for {
			var packet *Packet
			err := client.ReadJSON(&packet)
			if err != nil {
				c.Terminate()
				break
			}
			go c.HandleMessage(packet)
		}
	}()
}

func (w *WSServer) Listen() {
	http.Handle("/ws", &SocketHandler{})
	http.Handle("/metrics", &MetricsHandler{})
	logrus.Infof("Starting to listen on localhost:3010")
	if err := http.ListenAndServe("0.0.0.0:3010", nil); err != nil {
		logrus.Fatalf("HTTP Listen error: %v", err)
	}
}
