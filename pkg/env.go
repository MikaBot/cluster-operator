package pkg

import (
	"os"
	"strconv"
)

const ShardThresh int = 5

var (
	overwroteShards int
)

func GetAuth() string {
	return os.Getenv("AUTH")
}

func GetClusterCount() int {
	n, _ := strconv.Atoi(os.Getenv("CLUSTERS"))
	return n
}

func GetShardCount() int {
	if overwroteShards > 0 {
		return overwroteShards
	}
	n, _ := strconv.Atoi(os.Getenv("SHARDS"))
	return n
}
