package pkg

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"net/http"
	"strings"
	"time"
)

type EntityHandler struct{}

func (_ *EntityHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJson(w, 405, ApiResponse{Error: true, Message: "Method not allowed"})
		return
	}
	if r.Header.Get("Authorization") == "" {
		writeJson(w, 403, ApiResponse{Error: true, Message: "Unauthorized"})
		return
	}
	if r.Header.Get("Authorization") != GetAuth() {
		writeJson(w, 403, ApiResponse{Error: true, Message: "Forbidden"})
		return
	}
	if strings.Index(r.Header.Get("Content-Type"), "application/json") == -1 {
		writeJson(w, 400, ApiResponse{Error: true, Message: "Content-Type either not found, or not application/json!"})
		return
	}
	body := &EntityRequest{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		logrus.Errorf("Failed to decode JSON body for entity request: %s", err.Error())
		writeJson(w, 500, ApiResponse{Error: true, Message: "Unable to decode JSON body!"})
		return
	}
	if body.ID == "" {
		writeJson(w, 400, ApiResponse{Error: true, Message: "ID not specified!"})
		return
	}
	if body.Type == "" {
		writeJson(w, 400, ApiResponse{Error: true, Message: "type not specified!"})
		return
	}
	Server.PutEntityChan(body.ID)
	results := make([]EntityResponse, 0, len(Server.Clients))
	for _, cluster := range Server.Clients {
		if cluster.State == ClusterWaiting || cluster.State == ClusterConnecting {
			results = append(results, EntityResponse{Error: "cluster unhealthy"})
			continue
		}
		cluster.Write(Entity, body)
		select {
		case resp := <-Server.GetEntityChan(body.ID):
			{
				results = append(results, EntityResponse{
					ID:    "",
					Error: resp.Error,
					Data:  resp.Data,
				})
				break
			}
		case <-time.After(5 * time.Second):
			{
				results = append(results, EntityResponse{Error: "timed out"})
				break
			}
		}
	}
	Server.DeleteEntityChan(body.ID)
	writeJson(w, 200, ApiResponse{Data: results})
}
