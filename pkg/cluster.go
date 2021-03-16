package pkg

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type ClusterBlock struct {
	Shards []int `json:"shards"`
	Total  int   `json:"total"`
}

func getClusterBlock(id int) ClusterBlock {
	return ClusterBlock{
		Shards: Server.Clusters[id],
		Total:  GetShardCount(),
	}
}

type Cluster struct {
	index       int
	Terminated  bool
	ID          int
	Client      *websocket.Conn
	pingRecv    bool
	pingTicker  *time.Ticker
	statsTicker *time.Ticker
	mutex       *sync.Mutex
	evalChan    chan *EvalRes
}

type ClusterStats struct {
	Uptime        int            `json:"uptime"`
	Servers       int            `json:"servers"`
	Users         int            `json:"users"`
	Shards        int            `json:"shards"`
	ReadyShards   int            `json:"readyShards"`
	MemoryUsage   float64        `json:"memoryUsage"`
	MessagesSeen  int            `json:"messagesSeen"`
	CommandErrors map[string]int `json:"commandErrors"`
	CommandUsage  map[string]int `json:"commandUsage"`
}

type BroadcastEvalRequest struct {
	ID   string `json:"id"`
	Code string `json:"code"`
}

type BroadcastEvalResponse struct {
	ID      string     `json:"id"`
	Results []*EvalRes `json:"results"`
}

type EvalRes struct {
	Res   string `json:"res,omitempty"`
	Error string `json:"error,omitempty"`
}

func (c *Cluster) Terminate() {
	c.TerminateWithReason(0, "")
}

func (c *Cluster) TerminateWithReason(code int, reason string) {
	if c.Terminated {
		return
	}
	logrus.Infof("Terminating cluster %d ", c.ID)
	c.Terminated = true
	if c.pingTicker != nil {
		c.pingTicker.Stop()
	}
	if c.statsTicker != nil {
		c.statsTicker.Stop()
	}
	if code > 0 && len(reason) > 0 {
		_ = c.Client.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason))
	}
	_ = c.Client.Close()
	Server.Clients = append(Server.Clients[:c.index], Server.Clients[c.index+1:]...)
}

func (c *Cluster) HandleMessage(msg *Packet) {
	switch msg.Type {
	case Handshaking:
		lock.Lock()
		if num, ok := msg.Body.(float64); ok {
			if num < 0 || int(num) > len(Server.Clusters) {
				_ = c.Client.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(4001, "Cluster ID out of range"))
				_ = c.Client.Close()
				lock.Unlock()
				break
			}
			block := getClusterBlock(int(num))
			last := 0
			if len(block.Shards) < 2 {
				last = 1
			} else {
				last = block.Shards[len(block.Shards)-1]
			}
			if len(block.Shards) > ShardThresh {
				logrus.Warnf("Cluster %d will be handling more than %d shards, consider increasing cluster count!", int(num), ShardThresh)
			}
			c.ID = int(num)
			c.StartStatsCollector()
			c.StartHealthCheck()
			logrus.Infof("Giving cluster %d shards %d to %d", int(num), block.Shards[0], last)
			c.Write(ShardData, block)
			lock.Unlock()
			break
		}
		lock.Unlock()
		break
	case StatsAck:
		bytes, err := json.Marshal(msg.Body)
		if err != nil {
			break
		}
		stats := &ClusterStats{}
		err = json.Unmarshal(bytes, &stats)
		if err != nil {
			break
		}
		clusterMetrics = append(clusterMetrics, stats)
		break
	case PingAck:
		c.pingRecv = true
		break
	case BroadcastEval:
		{
			bytes, err := json.Marshal(msg.Body)
			if err != nil {
				break
			}
			req := &BroadcastEvalRequest{}
			err = json.Unmarshal(bytes, &req)
			if err != nil {
				break
			}
			results := make([]*EvalRes, 0, len(Server.Clients))
			for _, cluster := range Server.Clients {
				cluster.Write(Eval, req.Code)
				resp := <-cluster.evalChan
				results = append(results, resp)
			}
			c.Write(BroadcastEvalAck, BroadcastEvalResponse{
				ID:      req.ID,
				Results: results,
			})
		}
	case Eval:
		bytes, err := json.Marshal(msg.Body)
		if err != nil {
			break
		}
		res := &EvalRes{}
		err = json.Unmarshal(bytes, &res)
		if err != nil {
			break
		}
		c.evalChan <- res
	}
}

func (c *Cluster) Write(t int, data interface{}) {
	c.mutex.Lock()
	msg, _ := json.Marshal(Packet{
		Type: t,
		Body: data,
	})
	_ = c.Client.WriteMessage(websocket.TextMessage, msg)
	c.mutex.Unlock()
}

func (c *Cluster) StartStatsCollector() {
	c.statsTicker = time.NewTicker(15 * time.Second)
	go func() {
		for {
			select {
			case <-c.statsTicker.C:
				{
					c.Write(Stats, nil)
				}
			}
		}
	}()
}

func (c *Cluster) StartHealthCheck() {
	c.pingTicker = time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-c.pingTicker.C:
				{
					if !c.pingRecv {
						logrus.Warnf("Cluster %d has not responded to the last ping, terminating connection...", c.ID)
						c.TerminateWithReason(4001, "No ping received")
					}
					c.pingRecv = false
					c.Write(Ping, nil)
				}
			}
		}
	}()
}