package server

import (
	"encoding/json"
	"net/http"

	"github.com/IsaacJootar/kladd/backend/internal/config"
)

func NewRouter(cfg config.Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler(cfg))

	return mux
}

func healthHandler(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_ = json.NewEncoder(w).Encode(map[string]string{
			"service": "kladd-api",
			"status":  "ok",
			"addr":    cfg.HTTPAddr,
		})
	}
}
