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
	discordEvents = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: namedPrefix("discord_events"),
		Help: "Discord events per cluster",
	}, []string{"name"})
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
		discordEvents,
	)
}

func mergeClusterMetrics(clusterMetrics []*ClusterStats) *ClusterStats {
	mergedMetrics := &ClusterStats{
		CommandErrors: make(map[string]float64),
		CommandUsage:  make(map[string]float64),
		BotEvents:     make(map[string]float64),
	}
	for _, metrics := range clusterMetrics {
		mergedMetrics.Uptime += metrics.Uptime
		mergedMetrics.Servers += metrics.Servers
		mergedMetrics.Users += metrics.Users
		mergedMetrics.Shards += metrics.Shards
		mergedMetrics.ReadyShards += metrics.ReadyShards
		mergedMetrics.MessagesSeen += metrics.MessagesSeen
		for v, count := range metrics.CommandErrors {
			entry, ok := metrics.CommandErrors[v]
			if ok {
				entry += count
			} else {
				entry = count
			}
			mergedMetrics.CommandErrors[v] = count
		}
		for v, count := range metrics.CommandUsage {
			entry, ok := metrics.CommandUsage[v]
			if ok {
				entry += count
			} else {
				entry = count
			}
			mergedMetrics.CommandUsage[v] = count
		}
		for v, count := range metrics.BotEvents {
			entry, ok := metrics.BotEvents[v]
			if ok {
				entry += count
			} else {
				entry = count
			}
			mergedMetrics.BotEvents[v] = count
		}
	}
	return mergedMetrics
}

type MetricsHandler struct{}

func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if GetHealthyClusters() < 1 {
		// Reset metrics if no clusters are healthy rather than returning no data
		memoryUsage.Reset()
		commandErrors.Reset()
		commandUsage.Reset()
		servers.Set(0)
		users.Set(0)
		clusterCount.Set(0)
		shardCount.Set(0)
		messagesSeen.Set(0)
		memoryUsage.Reset()
		discordEvents.Reset()
		prometheusHandler.ServeHTTP(w, req)
		return
	}
	clusterMetrics := make([]*ClusterStats, 0, len(Server.Clients))
	for _, cluster := range Server.Clients {
		stats := cluster.RequestStats()
		if stats == nil {
			continue
		}
		clusterMetrics = append(clusterMetrics, stats)
		memoryUsage.With(prometheus.Labels{"cluster": strconv.Itoa(cluster.ID)}).Set(stats.MemoryUsage)
	}
	metrics := mergeClusterMetrics(clusterMetrics)
	for name, count := range metrics.CommandErrors {
		commandErrors.With(prometheus.Labels{"name": name}).Set(count)
	}
	for name, count := range metrics.CommandUsage {
		commandUsage.With(prometheus.Labels{"name": name}).Set(count)
	}
	for name, count := range metrics.BotEvents {
		discordEvents.With(prometheus.Labels{"name": name}).Set(count)
	}
	servers.Set(metrics.Servers)
	users.Set(metrics.Users)
	clusterCount.Set(float64(GetClusterCount()))
	shardCount.Set(metrics.ReadyShards)
	messagesSeen.Set(metrics.MessagesSeen)
	prometheusHandler.ServeHTTP(w, req)
}
