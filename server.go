package main

import (
	"fmt"
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
	Ready                   // client -> server
	Entity
	EntityAck
)

type WSServer struct {
	Clients        []*Cluster
	Upgrader       websocket.Upgrader
	ChanMutex      *sync.RWMutex
	Channels       map[string]chan EvalRes
	EntityMutex    *sync.RWMutex
	EntityChannels map[string]chan EntityResponse
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

func (w *WSServer) PutEntityChan(id string) {
	w.EntityMutex.Lock()
	w.EntityChannels[id] = make(chan EntityResponse)
	w.EntityMutex.Unlock()
}

func (w *WSServer) GetEntityChan(id string) chan EntityResponse {
	w.EntityMutex.RLock()
	val, ok := w.EntityChannels[id]
	if ok {
		w.EntityMutex.RUnlock()
		return val
	}
	w.EntityMutex.RUnlock()
	return nil
}

func (w *WSServer) DeleteEntityChan(id string) {
	w.EntityMutex.Lock()
	delete(w.EntityChannels, id)
	w.EntityMutex.Unlock()
}

func (*SocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	client, err := Server.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	if r.Header.Get("authorization") == "" || r.Header.Get("authorization") != Config.Auth {
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
	h := &MetricsHandler{}
	h.Setup()
	http.Handle("/ws", &SocketHandler{})
	http.Handle("/metrics", h)
	http.Handle("/eval", &EvalHandler{})
	http.Handle("/shardCount", &ExpectedShardHandler{})
	http.Handle("/entity", &EntityHandler{})
	http.Handle("/relay", &RelayHandler{
		mutex:   &sync.RWMutex{},
		clients: make(map[string]*websocket.Conn),
	})
	logrus.Infof("Starting to listen on %s:%d", Config.Ip, Config.Port)
	if err := http.ListenAndServe(fmt.Sprintf("%s:%d", Config.Ip, Config.Port), nil); err != nil {
		logrus.Fatalf("HTTP Listen error: %v", err)
	}
}
