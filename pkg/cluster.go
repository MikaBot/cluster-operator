package pkg

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type ClusterShardData struct {
	ID    int          `json:"id"`
	Block ClusterBlock `json:"block"`
}

type ClusterBlock struct {
	Shards []int `json:"shards"`
	Total  int   `json:"total"`
}

type ClusterState int

const (
	ClusterWaiting ClusterState = iota
	ClusterConnecting
	ClusterReady
)

type Cluster struct {
	ID         int             `json:"id"`
	Client     *websocket.Conn `json:"-"`
	PingRecv   bool            `json:"ping_recv"`
	Block      ClusterBlock    `json:"block"`
	State      ClusterState    `json:"state"`
	pingTicker *time.Ticker
	mutex      *sync.Mutex
	statsChan  chan map[string]interface{}
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
	BotEvents     map[string]float64 `json:"botEvents"`
}

type EntityRequest struct {
	ID   string                 `json:"id,omitempty"`
	Type string                 `json:"type"`
	Args map[string]interface{} `json:"args,omitempty"`
}

type EntityResponse struct {
	ID    string      `json:"id,omitempty"`
	Error string      `json:"error,omitempty"`
	Data  interface{} `json:"data,omitempty"`
}

type BroadcastEvalRequest struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	Timeout int    `json:"timeout"`
}

type BroadcastEvalResponse struct {
	ID      string    `json:"id"`
	Results []EvalRes `json:"results"`
}

type EvalRes struct {
	ID    string `json:"id,omitempty"`
	Res   string `json:"res,omitempty"`
	Error string `json:"error,omitempty"`
}

type ApiResponse struct {
	Error   bool        `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func (c *Cluster) Terminate() {
	c.TerminateWithReason(0, "", "disconnected")
}

func (c *Cluster) TerminateWithReason(code int, reason, logReason string) {
	logrus.Infof("Terminating cluster %d", c.ID)
	if c.pingTicker != nil {
		c.pingTicker.Stop()
	}
	if code > 0 && len(reason) > 0 {
		_ = c.Client.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason))
	}
	_ = c.Client.Close()
	if c.ID >= 0 && c.ID < len(Server.Clients) {
		Log.PostLog(c, ColorDisconnecting, logReason)
	}
	c.State = ClusterWaiting
	c.pingTicker = nil
}

func (c *Cluster) FirstShardID() int {
	return c.Block.Shards[0]
}

func (c *Cluster) LastShardID() int {
	return c.Block.Shards[len(c.Block.Shards)-1] + 1
}

func (c *Cluster) HandleMessage(msg *Packet) {
	switch msg.Type {
	case Handshaking:
		lock.Lock()
		c.StartHealthCheck()
		logrus.Infof("Giving cluster %d shards %d to %d", c.ID, c.FirstShardID(), c.LastShardID())
		Log.PostLog(c, ColorConnecting, "connecting")
		c.Write(ShardData, ClusterShardData{ID: c.ID, Block: c.Block})
		lock.Unlock()
		break
	case StatsAck:
		bytes, err := json.Marshal(msg.Body)
		if err != nil {
			break
		}
		stats := make(map[string]interface{})
		err = json.Unmarshal(bytes, &stats)
		if err != nil {
			break
		}
		c.statsChan <- stats
		break
	case PingAck:
		c.PingRecv = true
		break
	case Ready:
		c.State = ClusterReady
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
			Server.PutEvalChan(req.ID)
			results := make([]EvalRes, 0, len(Server.Clients))
			for _, cluster := range Server.Clients {
				if cluster.State == ClusterWaiting || cluster.State == ClusterConnecting {
					results = append(results, EvalRes{
						Error: "Cluster is not ready!",
					})
					continue
				}
				cluster.Write(Eval, BroadcastEvalRequest{
					ID:      req.ID,
					Code:    req.Code,
					Timeout: -1,
				})
				select {
				case resp := <-Server.GetEvalChan(req.ID):
					{
						results = append(results, EvalRes{
							ID:    "",
							Res:   resp.Res,
							Error: resp.Error,
						})
						break
					}
				case <-time.After(time.Duration(req.Timeout) * time.Millisecond):
					{
						results = append(results, EvalRes{
							Error: "Response timed out",
						})
						break
					}
				}
			}
			Server.DeleteEvalChan(req.ID)
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
		res := EvalRes{}
		err = json.Unmarshal(bytes, &res)
		if err != nil {
			break
		}
		if c := Server.GetEvalChan(res.ID); c != nil {
			c <- res
		}
	case EntityAck:
		bytes, err := json.Marshal(msg.Body)
		if err != nil {
			break
		}
		res := EntityResponse{}
		err = json.Unmarshal(bytes, &res)
		if err != nil {
			break
		}
		if c := Server.GetEntityChan(res.ID); c != nil {
			c <- res
		}
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

func (c *Cluster) RequestStats() map[string]interface{} {
	if c.Client != nil {
		c.Write(Stats, nil)
		select {
		case stats := <-c.statsChan:
			return stats
		case <-time.After(5 * time.Second):
			return nil
		}
	}
	return nil
}

func (c *Cluster) StartHealthCheck() {
	c.pingTicker = time.NewTicker(5 * time.Second)
	go func() {
		for {
			if c.pingTicker == nil {
				return
			}
			select {
			case <-c.pingTicker.C:
				{
					if c.State == ClusterReady {
						if !c.PingRecv {
							logrus.Warnf("Cluster %d has not responded to the last ping, terminating connection...", c.ID)
							c.TerminateWithReason(4001, "No ping received", "unhealthy")
						}
						c.PingRecv = false
						c.Write(Ping, nil)
					}
				}
			}
		}
	}()
}
