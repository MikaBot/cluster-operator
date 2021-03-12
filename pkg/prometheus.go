package pkg

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"net/http"
)

var (
	commandErrors = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: namedPrefix("command_errors"),
		Help: "Unexpected command errors",
	}, []string{"name"})
	commandUsage = prometheus.NewCounterVec(prometheus.CounterOpts{
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
	messagesSeen = prometheus.NewCounter(prometheus.CounterOpts{
		Name: namedPrefix("messages"),
		Help: "Messages seen",
	})
	memoryUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: namedPrefix("memory_usage"),
		Help: "Memory usage per cluster",
	}, []string{"cluster"})
	registry          = prometheus.NewRegistry()
	prometheusHandler = promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	clusterMetrics    []*ClusterStats
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

type MetricsHandler struct{}

func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	for _, metric := range clusterMetrics {
		logrus.Info(metric)
	}
	//clusterMetrics = []*ClusterStats{}
	prometheusHandler.ServeHTTP(w, req)
}
