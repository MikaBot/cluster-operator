package pkg

import (
	"encoding/json"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"os"
	"sync"
)

func namedPrefix(key string) string {
	_ = godotenv.Load()
	return "mika_" + os.Getenv("ENV") + "_" + key
}

func CreateClusters(shards, clusters int) {
	shardIds := make([]int, 0, shards)
	for i := 0; i < shards; i++ {
		shardIds = append(shardIds, i)
	}
	avgShardsPerCluster := shards / clusters
	for i := 0; i < len(shardIds); i += avgShardsPerCluster {
		Server.Clients = append(Server.Clients, &Cluster{
			ID:         len(Server.Clients),
			Client:     nil,
			PingRecv:   false,
			Block:      ClusterBlock{Shards: shardIds[i : i+avgShardsPerCluster], Total: GetShardCount()},
			State:      ClusterWaiting,
			pingTicker: nil,
			mutex:      &sync.Mutex{},
			evalChan:   make(chan *EvalRes),
			statsChan:  make(chan *ClusterStats),
		})
	}
}

func NextClusterID() int {
	for index, cluster := range Server.Clients {
		v, _ := json.Marshal(cluster)
		logrus.Info(string(v))
		if cluster.State == ClusterWaiting {
			return index
		}
	}
	return -1
}
