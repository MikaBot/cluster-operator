package main

import (
	"cluster-operator/pkg"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
)

func init() {
	_ = godotenv.Load()
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})
}

func main() {
	pkg.RegisterMetrics()
	pkg.NewLogger()
	pkg.Server = &pkg.WSServer{
		Clients:  []*pkg.Cluster{},
		Upgrader: websocket.Upgrader{},
	}
	pkg.CreateClusters(pkg.GetShardCount(), pkg.GetClusterCount())
	pkg.Log.PostOperatorLog(pkg.ColorReady, fmt.Sprintf(
		"Operator is online and will be handling %d shards with %d clusters, that's about %d shard(s) per cluster!",
		pkg.GetShardCount(),
		pkg.GetClusterCount(),
		pkg.GetShardCount()/pkg.GetClusterCount(),
	))
	go pkg.Server.Listen()
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGABRT)
	s := <-c
	pkg.Log.PostOperatorLog(pkg.ColorDisconnecting, fmt.Sprintf("Operator is going offline, no further clusters can connect (exit code: %s)!", s))
}
