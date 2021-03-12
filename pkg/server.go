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
)

type WSServer struct {
	Clients  []*Cluster
	Upgrader websocket.Upgrader
	Clusters [][]int
}

type SocketHandler struct{}

type Packet struct {
	Type int         `json:"type"`
	Body interface{} `json:"body,omitempty"`
}

func (*SocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("authorization") == "" || r.Header.Get("authorization") != GetAuth() {
		return
	}
	client, err := Server.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		logrus.Warnf("Failed to upgrade a cluster client: %v", err)
	}
	cluster := &Cluster{
		ID:          -1,
		index:       -1,
		Terminated:  false,
		Client:      client,
		pingRecv:    false,
		pingTicker:  nil,
		statsTicker: nil,
		mutex:       &sync.Mutex{},
		evalChan:    make(chan *EvalRes),
	}
	Server.Clients = append(Server.Clients, cluster)
	cluster.index = len(Server.Clients) - 1
	go func() {
		for {
			var packet *Packet
			err := client.ReadJSON(&packet)
			if err != nil {
				cluster.Terminate()
				break
			}
			go cluster.HandleMessage(packet)
		}
	}()
}

func (server *WSServer) Listen() {
	http.Handle("/", &SocketHandler{})
	http.Handle("/metrics", &MetricsHandler{})
	logrus.Infof("Starting to listen on localhost:3010")
	if err := http.ListenAndServe(":3010", nil); err != nil {
		logrus.Fatalf("HTTP Listen error: %v", err)
	}
}
