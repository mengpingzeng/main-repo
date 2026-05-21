package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"ai_provider/config"
)

// Server 是 AI Provider 的 HTTP 服务端
type Server struct {
	mgr  *config.Manager
	mux  *http.ServeMux
	port int
}

// NewServer 创建一个新的 HTTP 服务
func NewServer(mgr *config.Manager, port int) *Server {
	s := &Server{
		mgr:  mgr,
		mux:  http.NewServeMux(),
		port: port,
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/api/models", s.handleListModels)
	s.mux.HandleFunc("/api/models/", s.handleGetModel)
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/api/config/status", s.handleConfigStatus)
}

// Start 启动 HTTP 服务
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("AI Provider HTTP server listening on %s", addr)
	return http.ListenAndServe(addr, corsMiddleware(s.mux))
}

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method not allowed")
		return
	}

	models := s.mgr.ListModels()
	if models == nil {
		models = []config.ModelInfo{}
	}

	if provider := r.URL.Query().Get("provider"); provider != "" {
		filtered := []config.ModelInfo{}
		for _, m := range models {
			if m.Provider == provider {
				filtered = append(filtered, m)
			}
		}
		models = filtered
	}

	writeJSON(w, 200, map[string]interface{}{
		"models": models,
		"count":  len(models),
	})
}

func (s *Server) handleGetModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method not allowed")
		return
	}

	modelID := strings.TrimPrefix(r.URL.Path, "/api/models/")

	for _, m := range s.mgr.ListModels() {
		if m.ID == modelID {
			writeJSON(w, 200, m)
			return
		}
	}

	writeError(w, 404, "model not found: "+modelID)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	providerCount := len(s.mgr.GetProviderConfig().Provider)
	modelCount := len(s.mgr.ListModels())
	status := "ok"
	if providerCount == 0 {
		status = "degraded"
	}

	writeJSON(w, 200, map[string]interface{}{
		"status":    status,
		"service":   "ai_provider",
		"providers": providerCount,
		"models":    modelCount,
	})
}

func (s *Server) handleConfigStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method not allowed")
		return
	}

	warnings := s.mgr.Validate()
	models := s.mgr.ListModels()

	providerCount := len(s.mgr.GetProviderConfig().Provider)

	writeJSON(w, 200, map[string]interface{}{
		"global_config": config.OpenCodeConfigPath(),
		"providers":     providerCount,
		"models":        models,
		"warnings":      warnings,
		"healthy":       len(warnings) == 0,
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
