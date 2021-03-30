package pkg

import (
	"github.com/joho/godotenv"
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
			ID:         i,
			Block:      ClusterBlock{Shards: shardIds[i : i+avgShardsPerCluster], Total: GetShardCount()},
			Client:     nil,
			pingRecv:   false,
			pingTicker: nil,
			mutex:      &sync.Mutex{},
			evalChan:   make(chan *EvalRes),
			statsChan:  make(chan *ClusterStats),
		})
	}
}

func NextClusterID() int {
	for index, cluster := range Server.Clients {
		if cluster.State == ClusterWaiting {
			return index
		}
	}
	return -1
}
