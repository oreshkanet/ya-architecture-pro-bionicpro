package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"reports-backend/internal/auth"
	"reports-backend/internal/config"
	"reports-backend/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const logPrefix = "[reports-backend]"

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	return n, err
}

func requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		dur := time.Since(start)
		reqID := middleware.GetReqID(r.Context())
		if reqID == "" {
			reqID = "-"
		}
		userID := r.Header.Get("X-User-Id")
		if userID == "" {
			userID = "-"
		}
		log.Printf("%s [req=%s] %s %s | X-User-Id=%s | %d %dB | %v",
			logPrefix, reqID, r.Method, r.URL.RequestURI(), userID, rec.status, rec.bytes, dur)
	})
}

type Server struct {
	cfg  *config.Config
	repo *repository.ReportRepository
}

func New(cfg *config.Config) (*Server, error) {
	repo, err := repository.New(cfg.OLAPDSN)
	if err != nil {
		log.Printf("%s repository init: %v", logPrefix, err)
		return nil, err
	}
	log.Printf("%s repository connected", logPrefix)
	return &Server{cfg: cfg, repo: repo}, nil
}

func (s *Server) ListenAndServe(addr string) error {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(requestLogMiddleware)
	r.Use(middleware.Recoverer)

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
			log.Printf("%s [auth] 401 missing X-User-Id path=%s", logPrefix, r.URL.Path)
			http.Error(w, "missing X-User-Id", http.StatusUnauthorized)
			return
		}
		log.Printf("%s [auth] X-User-Id=%s path=%s", logPrefix, userID, r.URL.Path)
		r = r.WithContext(auth.WithUserID(r.Context(), userID))
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		log.Printf("%s [reports] 401 no user_id in context", logPrefix)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	log.Printf("%s [reports] GetByUserID user=%s", logPrefix, userID)
	report, err := s.repo.GetByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("%s [reports] GetByUserID error user=%s: %v", logPrefix, userID, err)
		http.Error(w, "failed to get report", http.StatusInternalServerError)
		return
	}
	if report == nil {
		log.Printf("%s [reports] not found user=%s", logPrefix, userID)
		http.Error(w, "report not found", http.StatusNotFound)
		return
	}

	log.Printf("%s [reports] 200 user=%s period=%s..%s", logPrefix, userID, report.PeriodStart, report.PeriodEnd)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(report); err != nil {
		log.Printf("%s [reports] encode error: %v", logPrefix, err)
	}
}
