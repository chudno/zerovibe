// Package http — JSON API оркестратора (управляющая плоскость).
// Это сервис для платформы/CLI, не для конечных юзеров → JSON, не HTML.
package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/chudno/zerovibe/orchestrator/internal/domain"
	"github.com/chudno/zerovibe/orchestrator/internal/usecase"
)

// Server держит ядро оркестратора.
type Server struct {
	orch *usecase.Orchestrator
}

func NewServer(orch *usecase.Orchestrator) *Server { return &Server{orch: orch} }

// Routes собирает маршруты.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/apps", s.deployApp)
	mux.HandleFunc("GET /v1/apps", s.listApps)
	mux.HandleFunc("GET /v1/nodes", s.listNodes)
	mux.HandleFunc("GET /v1/deployments", s.listDeployments)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

type deployRequest struct {
	OwnerID   string `json:"owner_id"`
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
}

func (s *Server) deployApp(w http.ResponseWriter, r *http.Request) {
	var req deployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	dep, err := s.orch.DeployApp(r.Context(), req.OwnerID, req.Name, req.Subdomain)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dep)
}

func (s *Server) listApps(w http.ResponseWriter, r *http.Request) {
	v, err := s.orch.ListApps(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) listNodes(w http.ResponseWriter, r *http.Request) {
	v, err := s.orch.ListNodes(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) listDeployments(w http.ResponseWriter, r *http.Request) {
	v, err := s.orch.ListDeployments(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// fail мапит доменные ошибки в HTTP-коды (единая точка).
func fail(w http.ResponseWriter, err error) {
	var nf domain.ErrNotFound
	var ve domain.ErrValidation
	switch {
	case errors.As(err, &nf):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.As(err, &ve):
		writeErr(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrNoCapacity):
		writeErr(w, http.StatusServiceUnavailable, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка: "+err.Error())
	}
}
