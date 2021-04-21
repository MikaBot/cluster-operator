package pkg

import (
	"net/http"
)

type ExpectedShardHandler struct{}

func (_ *ExpectedShardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
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
	writeJson(w, 200, ApiResponse{Data: GetShardCount()})
}
