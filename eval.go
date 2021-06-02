package main

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"net/http"
	"strings"
	"time"
)

type EvalHandler struct{}

func writeJson(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (_ *EvalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJson(w, 405, ApiResponse{Error: true, Message: "Method not allowed"})
		return
	}
	if r.Header.Get("Authorization") == "" {
		writeJson(w, 403, ApiResponse{Error: true, Message: "Unauthorized"})
		return
	}
	if r.Header.Get("Authorization") != Config.Auth {
		writeJson(w, 403, ApiResponse{Error: true, Message: "Forbidden"})
		return
	}
	if strings.Index(r.Header.Get("Content-Type"), "application/json") == -1 {
		writeJson(w, 400, ApiResponse{Error: true, Message: "Content-Type either not found, or not application/json!"})
		return
	}
	body := &BroadcastEvalRequest{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		logrus.Errorf("Failed to decode JSON body for eval request: %s", err.Error())
		writeJson(w, 500, ApiResponse{Error: true, Message: "Unable to decode JSON body!"})
		return
	}
	if body.ID == "" {
		writeJson(w, 400, ApiResponse{Error: true, Message: "Eval ID not specified!"})
		return
	}
	if body.Code == "" {
		writeJson(w, 400, ApiResponse{Error: true, Message: "Code not specified!"})
		return
	}
	if body.Timeout == 0 {
		writeJson(w, 400, ApiResponse{Error: true, Message: "Timeout not specified!"})
		return
	}
	Server.PutEvalChan(body.ID)
	results := make([]EvalRes, 0, len(Server.Clients))
	for _, cluster := range Server.Clients {
		if cluster.State == ClusterWaiting || cluster.State == ClusterConnecting {
			results = append(results, EvalRes{Error: "Cluster is not ready!"})
			continue
		}
		cluster.Write(Eval, BroadcastEvalRequest{
			ID:      body.ID,
			Code:    body.Code,
			Timeout: -1,
		})
		select {
		case resp := <-Server.GetEvalChan(body.ID):
			{
				results = append(results, EvalRes{
					ID:    "",
					Res:   resp.Res,
					Error: resp.Error,
				})
				break
			}
		case <-time.After(time.Duration(body.Timeout) * time.Millisecond):
			{
				results = append(results, EvalRes{Error: "Response timed out"})
				break
			}
		}
	}
	writeJson(w, 200, ApiResponse{Data: results})
}
