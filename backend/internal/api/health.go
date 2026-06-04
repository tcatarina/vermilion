package api

import (
	"net/http"
	"time"
)

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Time    string `json:"time"`
}

func Healthz(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, HealthResponse{
		Status:  "ok",
		Service: "vermilion-backend",
		Time:    time.Now().UTC().Format(time.RFC3339),
	})
}

func NotFoundJSON(w http.ResponseWriter, r *http.Request) {
	NotFound(w, "route not found: "+r.Method+" "+r.URL.Path)
}

func MethodNotAllowedJSON(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusMethodNotAllowed, APIError{
		Code:    CodeInvalidRequest,
		Message: "method not allowed: " + r.Method + " " + r.URL.Path,
	})
}
