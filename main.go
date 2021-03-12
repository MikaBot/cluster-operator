package main

import (
	"cluster-operator/pkg"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func init() {
	_ = godotenv.Load()
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})
}

func main() {
	pkg.RegisterMetrics()
	groups := pkg.GetShardGroups(pkg.GetShardCount(), pkg.GetClusterCount())
	pkg.Server = &pkg.WSServer{
		Clients:  []*pkg.Cluster{},
		Upgrader: websocket.Upgrader{},
		Clusters: groups,
	}
	pkg.Server.Listen()
}
