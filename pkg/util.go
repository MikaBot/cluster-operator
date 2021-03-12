package pkg

import (
	"github.com/joho/godotenv"
	"os"
)

func namedPrefix(key string) string {
	_ = godotenv.Load()
	return "mika_" + os.Getenv("ENV") + "_" + key
}

func GetShardGroups(shards, clusters int) [][]int {
	shardIds := make([]int, 0, shards)
	for i := 0; i < shards; i++ {
		shardIds = append(shardIds, i)
	}
	chunks := make([][]int, 0, clusters)
	avgShardsPerCluster := shards / clusters
	for i := 0; i < len(shardIds); i += avgShardsPerCluster {
		chunks = append(chunks, shardIds[i:i+avgShardsPerCluster])
	}
	return chunks
}
