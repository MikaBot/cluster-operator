package pkg

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"strconv"
)

var (
	commandErrors = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: namedPrefix("command_errors"),
		Help: "Unexpected command errors",
	}, []string{"name"})
	commandUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: namedPrefix("command_usage"),
		Help: "Command usage",
	}, []string{"name"})
	servers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: namedPrefix("servers"),
		Help: "Server count",
	})
	users = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: namedPrefix("users"),
		Help: "User count",
	})
	clusterCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: namedPrefix("cluster_count"),
		Help: "Cluster count",
	})
	shardCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: namedPrefix("shard_count"),
		Help: "Shard count",
	})
	messagesSeen = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: namedPrefix("messages"),
		Help: "Messages seen",
	})
	memoryUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: namedPrefix("memory_usage"),
		Help: "Memory usage per cluster",
	}, []string{"cluster"})
	registry          = prometheus.NewRegistry()
	prometheusHandler = promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
)

func RegisterMetrics() {
	registry.MustRegister(
		commandErrors,
		commandUsage,
		servers,
		users,
		clusterCount,
		shardCount,
		messagesSeen,
		memoryUsage,
	)
}

func mergeClusterMetrics(clusterMetrics []*ClusterStats) *ClusterStats {
	mergedMetrics := &ClusterStats{
		CommandErrors: make(map[string]float64),
		CommandUsage:  make(map[string]float64),
	}
	for _, metrics := range clusterMetrics {
		mergedMetrics.Uptime += metrics.Uptime
		mergedMetrics.Servers += metrics.Servers
		mergedMetrics.Users += metrics.Users
		mergedMetrics.Shards += metrics.Shards
		mergedMetrics.ReadyShards += metrics.ReadyShards
		mergedMetrics.MessagesSeen += metrics.MessagesSeen
		for err, count := range metrics.CommandErrors {
			entry, ok := metrics.CommandErrors[err]
			if ok {
				entry += count
			} else {
				entry = count
			}
			mergedMetrics.CommandErrors[err] = count
		}
		for err, count := range metrics.CommandUsage {
			entry, ok := metrics.CommandUsage[err]
			if ok {
				entry += count
			} else {
				entry = count
			}
			mergedMetrics.CommandUsage[err] = count
		}
	}
	return mergedMetrics
}

type MetricsHandler struct{}

func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	clusterMetrics := make([]*ClusterStats, 0, len(Server.Clients))
	for _, cluster := range Server.Clients {
		stats := cluster.RequestStats()
		if stats == nil {
			continue
		}
		clusterMetrics = append(clusterMetrics, stats)
		g := memoryUsage.With(prometheus.Labels{"cluster": strconv.Itoa(cluster.ID)})
		g.Set(stats.MemoryUsage)
	}
	metrics := mergeClusterMetrics(clusterMetrics)
	for name, count := range metrics.CommandErrors {
		g := commandErrors.With(prometheus.Labels{"name": name})
		g.Set(count)
	}
	for name, count := range metrics.CommandUsage {
		g := commandUsage.With(prometheus.Labels{"name": name})
		g.Set(count)
	}
	servers.Set(metrics.Servers)
	users.Set(metrics.Users)
	clusterCount.Set(float64(GetClusterCount()))
	shardCount.Set(metrics.ReadyShards)
	messagesSeen.Set(metrics.MessagesSeen)
	prometheusHandler.ServeHTTP(w, req)
}
