package server

import (
	"encoding/json"
	"log"
	"net/http"

	"reports-backend/internal/config"
	"reports-backend/internal/repository"
	"reports-backend/internal/auth"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	cfg  *config.Config
	repo *repository.ReportRepository
}

func New(cfg *config.Config) (*Server, error) {
	repo, err := repository.New(cfg.OLAPDSN)
	if err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, repo: repo}, nil
}

func (s *Server) ListenAndServe(addr string) error {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	// GET /reports — вызывается только bionicpro-auth; в заголовке передаётся ID пользователя из сессии (токен на клиент не отдаётся).
	r.With(s.requireUserID).Get("/reports", s.handleGetReport)

	return http.ListenAndServe(addr, r)
}

// requireUserID: запрос приходит от auth-сервиса с заголовком X-User-Id (auth уже проверил сессию по cookie).
// Фильтрация данных — только по этому ID.
func (s *Server) requireUserID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-Id")
		if userID == "" {
			http.Error(w, "missing X-User-Id", http.StatusUnauthorized)
			return
		}
		r = r.WithContext(auth.WithUserID(r.Context(), userID))
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Запрос только по своему user_id — ограничение доступа (задача 4)
	report, err := s.repo.GetByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("[reports] GetByUserID %s: %v", userID, err)
		http.Error(w, "failed to get report", http.StatusInternalServerError)
		return
	}
	if report == nil {
		http.Error(w, "report not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(report); err != nil {
		log.Printf("[reports] encode: %v", err)
	}
}
