package pkg

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"os"
)

var Config OperatorConfig

type Metric struct {
	// The name of the metric (REQUIRED)
	Name string `json:"name"`
	// The metric type (REQUIRED)
	Type string `json:"type"`
	// What this metric is for (REQUIRED)
	Description string `json:"description"`
	// Optional labels
	Labels []string `json:"labels"`
}

type OperatorConfig struct {
	// The ip to bind to (optional, default 127.0.0.1)
	Ip string `json:"ip"`
	//The port to listen on (optional, default 3010)
	Port int `json:"port"`
	// The current environment used for logging purposes (REQUIRED)
	Env string `json:"env"`
	// The count of clusters that will be expected to connect in order to maintain maximum quorum. (REQUIRED)
	Clusters int `json:"clusters"`
	// The total amount of shards  (REQUIRED)
	Shards int `json:"shards"`
	// The authentication token to use (REQUIRED)
	Auth string `json:"auth"`
	// A discord webhook to use for posting cluster related logs
	Webhook string `json:"webhook"`
	// A prefix to use for metrics, which a metric would resolve to `prod_servers`.
	MetricsPrefix string `json:"metricsPrefix"`
	// An array of metrics that prometheus will scrape
	Metrics []Metric `json:"metrics"`
	// If metrics between clusters should be merged together, when this is false, the first clusters metrics will be used
	MergeMetrics bool `json:"mergeMetrics"`
	// If the cluster operator should log cluster events to the webhook (as defined above)
	LogEvents bool `json:"logEvents"`
}

func init() {
	cwd, _ := os.Getwd()
	file, err := os.ReadFile(cwd + "/config.json")
	if err != nil {
		logrus.Fatal("Failed to load config file!")
		return
	}
	err = json.Unmarshal(file, &Config)
	if err != nil {
		logrus.Fatal("Failed to decode the config file, this could be that the file is corrupt, or malformed!")
		return
	}
	if Config.Ip == "" {
		Config.Ip = "127.0.0.1"
	}
	if Config.Port == 0 {
		Config.Port = 3010
	}
	if Config.Env == "" && Config.LogEvents {
		logrus.Fatal("An env name should be set when you're using logs!")
	}
	if Config.Clusters < 1 {
		logrus.Fatal("Cluster count should be greater than 0!")
	}
	if Config.Shards < 1 {
		logrus.Fatal("Shard count should be greater than 0!")
	}
	if len(Config.Metrics) > 0 && Config.MetricsPrefix == "" {
		logrus.Warn("You have set multiple metrics, but no metrics prefix; ignore this warning if you know what you're doing! However, a metric with the name 'ping' can be overwritten by any other cluster operators that run on your server!")
	}
	if len(Config.Metrics) > 0 {
		for i, metric := range Config.Metrics {
			if metric.Name == "" {
				logrus.Fatalf("metrics[%d].name is a required field!", i)
			}
			if metric.Description == "" {
				logrus.Fatalf("metrics[%d].description is a required field!", i)
			}
			if metric.Type == "" {
				logrus.Fatalf("metrics[%d].type is a required field!", i)
			}
			if !(metric.Type == "gauge" || metric.Type == "counter") {
				logrus.Fatalf("metrics[%d].type should be gauge or collector, received: %s!", i, metric.Type)
			}
		}
	}
	logrus.Info("Found and loaded config.json!")
}

func MetricPrefix(key string) string {
	return Config.MetricsPrefix + key
}
