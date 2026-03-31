package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"
)

//go:embed web/templates/*.html web/static/*.css
var webFiles embed.FS

type Server struct {
	registry *Registry
	updater  *Updater
	tmpl     *template.Template
}

func NewServer(registry *Registry, updater *Updater) (*Server, error) {
	tmpl, err := template.ParseFS(webFiles, "web/templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}
	return &Server{
		registry: registry,
		updater:  updater,
		tmpl:     tmpl,
	}, nil
}

func (s *Server) Start(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/api/resources", s.handleListResources)
	mux.HandleFunc("/api/resources/approve", s.handleApproveUpdate)
	mux.HandleFunc("/health", s.handleHealth)

	staticFS, _ := fs.Sub(webFiles, "web/static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	server := &http.Server{
		Addr:    addr,
		Handler: s.loggingMiddleware(mux),
	}

	go func() {
		<-ctx.Done()
		slog.Info("🛑 Shutting down HTTP server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server shutdown error", "error", err)
		}
	}()

	slog.Info("🌐 HTTP server starting", "addr", addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}
	return nil
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	resources := s.registry.GetAllResources()
	if err := s.tmpl.ExecuteTemplate(w, "dashboard.html", resources); err != nil {
		slog.Error("Template execution error", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (s *Server) handleListResources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resources := s.registry.GetAllResources()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resources); err != nil {
		slog.Error("JSON encoding error", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (s *Server) handleApproveUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Namespace string `json:"namespace"`
		Kind      string `json:"kind"`
		Name      string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.approveAndUpdate(req.Namespace, req.Name, req.Kind); err != nil {
		slog.Error("Approval failed", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (s *Server) approveAndUpdate(namespace, name, kind string) error {
	if err := s.registry.ApproveUpdate(namespace, name, kind); err != nil {
		return err
	}
	res, exists := s.registry.Get(namespace, name, kind)
	if !exists {
		return fmt.Errorf("resource not found after approval")
	}
	if err := s.updater.TriggerRollingUpdate(res); err != nil {
		return err
	}
	slog.Info("✅ Update approved and triggered",
		"namespace", namespace,
		"kind", kind,
		"name", name,
	)
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("HTTP request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
