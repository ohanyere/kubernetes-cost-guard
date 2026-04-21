package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ohanyere/kubernetes-cost-guard/internal/config"
	"github.com/ohanyere/kubernetes-cost-guard/internal/kube"
)

type Server struct {
	cfg    config.Config
	logger *slog.Logger
	kube   kube.Reader
}

func NewServer(cfg config.Config, logger *slog.Logger, kubeReader kube.Reader) http.Handler {
	server := &Server{
		cfg:    cfg,
		logger: logger,
		kube:   kubeReader,
	}

	return server.routes()
}

func (s *Server) routes() http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(s.requestLogger)

	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	})

	router.Get("/healthz", s.handleHealth)

	router.Route("/api/v1", func(r chi.Router) {
		r.Post("/analyze/workload", s.handleAnalyzeWorkload)
		r.Post("/analyze/namespace", s.handleAnalyzeNamespace)
		r.Post("/recommend/dry-run", s.handleRecommendDryRun)
		r.Post("/recommend/apply", s.handleRecommendApply)
		r.Get("/history", s.handleHistory)
	})

	return router
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":      "ok",
		"service":     s.cfg.AppName,
		"environment": s.cfg.Environment,
		"version":     s.cfg.Version,
		"time":        time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("request completed",
			"request_id", middleware.GetReqID(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start).String(),
		)
	})
}
