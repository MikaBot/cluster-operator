package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors: true,
	})
	logrus.SetLevel(logrus.DebugLevel)
}

func main() {
	NewLogger()
	Server = &WSServer{
		Clients:        []*Cluster{},
		Upgrader:       websocket.Upgrader{},
		ChanMutex:      &sync.RWMutex{},
		Channels:       make(map[string]chan EvalRes),
		EntityMutex:    &sync.RWMutex{},
		EntityChannels: make(map[string]chan EntityResponse),
	}
	CreateClusters(Config.Shards, Config.Clusters)
	Log.PostOperatorLog(ColorReady, fmt.Sprintf(
		"Operator is online and will be handling %d shards with %d clusters, that's about %d shard(s) per cluster!",
		Config.Shards,
		Config.Clusters,
		Config.Shards/Config.Clusters,
	))
	go Server.Listen()
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGABRT)
	s := <-c
	Log.PostOperatorLog(ColorDisconnecting, fmt.Sprintf("Operator is going offline, no further clusters can connect (exit code: %s)!", s))
}
