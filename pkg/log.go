package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"
	"time"
)

const (
	ColorConnecting    = 0x0E6CF7
	ColorReady         = 0x00DB62
	ColorDisconnecting = 0xFF4444
)

type Logger struct {
	webhookUrl string
	client     *http.Client
}

var (
	Log *Logger
)

type Embed struct {
	Color       int    `json:"color"`
	Description string `json:"description"`
}

type WebhookBody struct {
	Embeds []Embed `json:"embeds"`
}

func NewLogger() {
	if Log != nil {
		panic("Tried to initialise another logger instance.")
	}
	Log = &Logger{
		webhookUrl: Config.Webhook,
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return nil
			},
		},
	}
	logrus.Info("Created a logger instance!")
}

func (log *Logger) makeRequest(embed Embed) (*http.Request, error) {
	marshaled, err := json.Marshal(WebhookBody{
		Embeds: []Embed{embed},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", log.webhookUrl, bytes.NewBuffer(marshaled))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", fmt.Sprintf("DiscordBot (%s, 1.0.0)", "https://mikabot.gg"))
	return req, nil
}

func (log *Logger) currentDate() string {
	now := time.Now()
	return fmt.Sprintf(
		"%s %02d %02d %02d:%02d:%02d",
		now.Month().String(),
		now.Day(),
		now.Year(),
		now.Hour(),
		now.Minute(),
		now.Second(),
	)
}

func (log *Logger) description(c *Cluster, event string) string {
	return fmt.Sprintf(
		"`[%s]` | Cluster `%d` %s | Shards `%d` - `%d` | %s",
		log.currentDate(),
		c.ID,
		event,
		c.FirstShardID(),
		c.LastShardID(),
		Config.Env,
	)
}

func (log *Logger) operator(event string) string {
	return fmt.Sprintf(
		"`[%s]` | %s | %s",
		log.currentDate(),
		event,
		Config.Env,
	)
}

func (log *Logger) PostLog(c *Cluster, color int, event string) {
	if !Config.LogEvents {
		return
	}
	s := time.Now()
	req, err := log.makeRequest(Embed{
		Color:       color,
		Description: log.description(c, event),
	})
	if err != nil {
		logrus.Errorf("PostLog failed to make a request: %s", err.Error())
		return
	}
	res, err := log.client.Do(req)
	if err != nil {
		logrus.Errorf("PostLog failed to get a response: %s", err.Error())
		return
	}
	logrus.Debugf("Got status %s from Discord in %s, when firing event %s!", res.Status, time.Now().Sub(s).String(), event)
}

func (log *Logger) PostOperatorLog(color int, event string) {
	if !Config.LogEvents {
		return
	}
	s := time.Now()
	req, err := log.makeRequest(Embed{
		Color:       color,
		Description: log.operator(event),
	})
	if err != nil {
		logrus.Errorf("PostOperatorLog failed to make a request: %s", err.Error())
		return
	}
	res, err := log.client.Do(req)
	if err != nil {
		logrus.Errorf("PostOperatorLog failed to get a response: %s", err.Error())
		return
	}
	logrus.Debugf("Got status %s from Discord in %s, when posting operator log!", res.Status, time.Now().Sub(s).String())
}
