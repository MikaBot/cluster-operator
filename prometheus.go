package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"net/http"
	"reflect"
	"strconv"
)

type Collector struct {
	metric   string
	original prometheus.Collector
}

type MetricsHandler struct {
	metrics      []Collector
	clusterCount prometheus.Gauge
	shardCount   prometheus.Gauge
}

var (
	registry          = prometheus.NewRegistry()
	prometheusHandler = promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
)

func findMetricByName(name string) *Metric {
	for _, metric := range Config.Metrics {
		if metric.Name == name {
			return &metric
		}
	}
	return nil
}

// Setup registers all metrics that the user has defined in their config.json
// This also exposes 2 default metrics, cluster_count and shard_count
func (h *MetricsHandler) Setup() {
	h.clusterCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricPrefix("cluster_count"),
		Help: "Total clusters!",
	})
	h.shardCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricPrefix("shard_count"),
		Help: "Total shards!",
	})
	registry.MustRegister(h.clusterCount, h.shardCount)
	for _, metric := range Config.Metrics {
		var collector prometheus.Collector
		if metric.Type == "gauge" && len(metric.Labels) > 0 {
			collector = prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: MetricPrefix(metric.Name),
				Help: metric.Description,
			}, metric.Labels)
		} else if metric.Type == "gauge" && len(metric.Labels) < 1 {
			collector = prometheus.NewGauge(prometheus.GaugeOpts{
				Name: MetricPrefix(metric.Name),
				Help: metric.Description,
			})
		} else if metric.Type == "counter" && len(metric.Labels) > 0 {
			collector = prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: MetricPrefix(metric.Name),
				Help: metric.Description,
			}, metric.Labels)
		} else if metric.Type == "counter" && len(metric.Labels) < 1 {
			collector = prometheus.NewCounter(prometheus.CounterOpts{
				Name: MetricPrefix(metric.Name),
				Help: metric.Description,
			})
		}
		h.metrics = append(h.metrics, Collector{
			metric:   metric.Name,
			original: collector,
		})
		if err := registry.Register(collector); err == nil {
			logrus.Infof("Picked up and registered metric %s as type %s", metric.Name, metric.Type)
		} else {
			logrus.Errorf("Failed to register metric %s: %s", metric.Name, err.Error())
		}
	}
}

// This function will merge all cluster metrics into a single map of objects.
// Just a reminder that, anything as a nested object will be treated as a LABELED metric, and expects it's children stats to also be maps with a number as it's value.
func (h *MetricsHandler) mergeMetrics(metrics []map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})
	for i, metric := range metrics {
		for key, child := range metric {
			m := findMetricByName(key)
			if m == nil {
				logrus.Warnf("Cluster %d has an unknown metric field %s!", i, key)
				continue
			}
			if f, ok := child.(float64); ok {
				if len(m.Labels) > 0 && m.Labels[0] == "cluster" {
					if _, ok := merged[key]; ok {
						merged[key].(map[string]interface{})[strconv.Itoa(i)] = f
					} else {
						merged[key] = map[string]interface{}{strconv.Itoa(i): f}
					}
				} else {
					if v, ok := merged[key]; ok {
						merged[key] = v.(float64) + f
					} else {
						merged[key] = f
					}
				}
			}
			if m, ok := child.(map[string]interface{}); ok {
				for k, v := range m {
					if f, ok := v.(float64); ok {
						if mergedMap, ok := merged[key].(map[string]interface{}); ok {
							if v1, ok := mergedMap[k]; ok {
								mergedMap[k] = v1.(float64) + f
							} else {
								mergedMap[k] = f
							}
							merged[key] = mergedMap
						} else {
							merged[key] = map[string]interface{}{
								k: f,
							}
						}
					} else {
						logrus.Errorf("Expected key %s to be an integer, got %s instead!", k, reflect.TypeOf(v).Name())
					}
				}
			}
		}
	}
	return merged
}

func (h *MetricsHandler) findCollector(name string) *Collector {
	for _, c := range h.metrics {
		if c.metric == name {
			return &c
		}
	}
	return nil
}

func (h *MetricsHandler) setCollector(labels prometheus.Labels, val float64, c *Collector) {
	if v, ok := c.original.(prometheus.Gauge); ok {
		v.Set(val)
	} else if v, ok := c.original.(*prometheus.GaugeVec); ok {
		v.With(labels).Set(val)
	} else if v, ok := c.original.(prometheus.Counter); ok {
		v.Add(val)
	} else if v, ok := c.original.(*prometheus.CounterVec); ok {
		v.With(labels).Add(val)
	}
}

func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if GetHealthyClusters() < 1 {
		w.WriteHeader(500)
		return
	}
	clusterMetrics := make([]map[string]interface{}, 0, len(Server.Clients))
	for _, cluster := range Server.Clients {
		stats := cluster.RequestStats()
		if stats == nil {
			continue
		}
		clusterMetrics = append(clusterMetrics, stats)
	}
	metrics := make(map[string]interface{})
	if !Config.MergeMetrics {
		metrics = clusterMetrics[0]
	} else {
		metrics = h.mergeMetrics(clusterMetrics)
	}
	for key, child := range metrics {
		c := h.findCollector(key)
		if c == nil {
			continue
		}
		m := findMetricByName(c.metric)
		if m == nil {
			continue
		}
		if f, ok := child.(float64); ok {
			h.setCollector(nil, f, c)
		}
		if data, ok := child.(map[string]interface{}); ok {
			for key, v := range data {
				f := v.(float64)
				labels := prometheus.Labels{}
				for _, label := range m.Labels {
					labels[label] = key
				}
				h.setCollector(labels, f, c)
			}
		}
	}
	prometheusHandler.ServeHTTP(w, req)
}
