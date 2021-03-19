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
	statsChan   chan *ClusterStats
	Block       ClusterBlock
}

type ClusterStats struct {
	Uptime        float64            `json:"uptime"`
	Servers       float64            `json:"servers"`
	Users         float64            `json:"users"`
	Shards        float64            `json:"shards"`
	ReadyShards   float64            `json:"readyShards"`
	MemoryUsage   float64            `json:"memoryUsage"`
	MessagesSeen  float64            `json:"messagesSeen"`
	CommandErrors map[string]float64 `json:"commandErrors"`
	CommandUsage  map[string]float64 `json:"commandUsage"`
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
	c.TerminateWithReason(0, "", "disconnected")
}

func (c *Cluster) TerminateWithReason(code int, reason, logReason string) {
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
	if c.ID >= 0 && c.ID < len(Server.Clusters) {
		Log.PostLog(c, ColorDisconnecting, logReason)
	} else {
		Log.PostOperatorLog(ColorDisconnecting, "Disconnected bad cluster")
	}
	Server.Clients = append(Server.Clients[:c.index], Server.Clients[c.index+1:]...)
}

func (c *Cluster) FirstShardID() int {
	return c.Block.Shards[0]
}

func (c *Cluster) LastShardID() int {
	if len(c.Block.Shards) < 2 {
		return 1
	} else {
		return c.Block.Shards[len(c.Block.Shards)-1]
	}
}

func (c *Cluster) HandleMessage(msg *Packet) {
	switch msg.Type {
	case Handshaking:
		lock.Lock()
		if num, ok := msg.Body.(float64); ok {
			logrus.Info(int(num), len(Server.Clusters))
			if int(num) >= len(Server.Clusters) {
				_ = c.Client.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(4001, "Cluster ID out of range"))
				_ = c.Client.Close()
				lock.Unlock()
				break
			}
			c.Block = getClusterBlock(int(num))
			if len(c.Block.Shards) > ShardThresh {
				logrus.Warnf("Cluster %d will be handling more than %d shards, consider increasing cluster count!", int(num), ShardThresh)
			}
			c.ID = int(num)
			c.StartHealthCheck()
			logrus.Infof("Giving cluster %d shards %d to %d", int(num), c.FirstShardID(), c.LastShardID())
			Log.PostLog(c, ColorConnecting, "connecting")
			c.Write(ShardData, c.Block)
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
		c.statsChan <- stats
		break
	case PingAck:
		c.pingRecv = true
		break
	case Ready:
		Log.PostLog(c, ColorReady, "ready")
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

func (c *Cluster) RequestStats() *ClusterStats {
	c.Write(Stats, nil)
	select {
	case stats := <-c.statsChan:
		return stats
	case <-time.After(5 * time.Second):
		return nil
	}
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
						c.TerminateWithReason(4001, "No ping received", "unhealthy")
					}
					c.pingRecv = false
					c.Write(Ping, nil)
				}
			}
		}
	}()
}
