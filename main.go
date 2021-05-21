package main

import (
	"cluster-operator/pkg"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

func main() {
	pkg.NewLogger()
	pkg.Server = &pkg.WSServer{
		Clients:        []*pkg.Cluster{},
		Upgrader:       websocket.Upgrader{},
		ChanMutex:      &sync.RWMutex{},
		Channels:       make(map[string]chan pkg.EvalRes),
		EntityMutex:    &sync.RWMutex{},
		EntityChannels: make(map[string]chan pkg.EntityResponse),
	}
	pkg.CreateClusters(pkg.Config.Shards, pkg.Config.Clusters)
	pkg.Log.PostOperatorLog(pkg.ColorReady, fmt.Sprintf(
		"Operator is online and will be handling %d shards with %d clusters, that's about %d shard(s) per cluster!",
		pkg.Config.Shards,
		pkg.Config.Clusters,
		pkg.Config.Shards/pkg.Config.Clusters,
	))
	go pkg.Server.Listen()
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGABRT)
	s := <-c
	pkg.Log.PostOperatorLog(pkg.ColorDisconnecting, fmt.Sprintf("Operator is going offline, no further clusters can connect (exit code: %s)!", s))
}
