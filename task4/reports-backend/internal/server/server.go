package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"reports-backend/internal/auth"
	"reports-backend/internal/config"
	"reports-backend/internal/report"
	"reports-backend/internal/reporttoken"
	"reports-backend/internal/repository"
	"reports-backend/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const logPrefix = "[reports-backend]"

func reportS3Key(userID, periodStart, periodEnd string) string {
	return fmt.Sprintf("reports/%s/%s_%s.html", storage.UserIDHashPrefix(userID), periodStart, periodEnd)
}

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
	cfg     *config.Config
	repo    *repository.ReportRepository
	storage *storage.S3Storage
}

func New(cfg *config.Config) (*Server, error) {
	repo, err := repository.New(cfg.ClickHouseDSN)
	if err != nil {
		log.Printf("%s repository init: %v", logPrefix, err)
		return nil, err
	}
	log.Printf("%s repository connected", logPrefix)
	s3Storage, err := storage.NewS3(cfg.S3Endpoint, cfg.S3Bucket, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3ReportTTLDays)
	if err != nil {
		log.Printf("%s S3 storage init: %v", logPrefix, err)
		return nil, err
	}
	return &Server{cfg: cfg, repo: repo, storage: s3Storage}, nil
}

func (s *Server) ListenAndServe(addr string) error {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(requestLogMiddleware)
	r.Use(middleware.Recoverer)

	r.With(s.requireUserID).Get("/reports", s.handleGetReport)
	r.With(s.requireUserID).Get("/reports/serve", s.handleServeReport)

	return http.ListenAndServe(addr, r)
}

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

	ctx := r.Context()

	// Период выбираем из БД (лёгкий запрос) - период для ключа S3. В целом указание периода можно вынести в UI пользователя.
	periodStart, periodEnd, err := s.repo.GetPeriodByUserID(ctx, userID)
	if err != nil {
		log.Printf("%s [reports] GetPeriodByUserID error user=%s: %v", logPrefix, userID, err)
		http.Error(w, "failed to get report", http.StatusInternalServerError)
		return
	}
	if periodStart == "" || periodEnd == "" {
		log.Printf("%s [reports] not found user=%s", logPrefix, userID)
		http.Error(w, "report not found", http.StatusNotFound)
		return
	}

	key := reportS3Key(userID, periodStart, periodEnd)

	// Проверка наличия отчёта в S3
	exists, err := s.storage.Exists(ctx, key)
	if err != nil {
		log.Printf("%s [reports] S3 Exists error user=%s: %v", logPrefix, userID, err)
		http.Error(w, "failed to check report", http.StatusInternalServerError)
		return
	}
	if !exists {
		reportRow, err := s.repo.GetByUserID(ctx, userID)
		if err != nil {
			log.Printf("%s [reports] GetByUserID error user=%s: %v", logPrefix, userID, err)
			http.Error(w, "failed to get report", http.StatusInternalServerError)
			return
		}
		if reportRow == nil {
			http.Error(w, "report not found", http.StatusNotFound)
			return
		}
		htmlBody := report.RenderHTML(reportRow)
		if err := s.storage.Put(ctx, key, []byte(htmlBody), "text/html; charset=utf-8"); err != nil {
			log.Printf("%s [reports] S3 Put error user=%s: %v", logPrefix, userID, err)
			http.Error(w, "failed to save report", http.StatusInternalServerError)
			return
		}
		log.Printf("%s [reports] generated and saved to S3 user=%s key=%s", logPrefix, userID, key)
	}

	// Формирование защищённой ссылки на отчёт
	tok := reporttoken.Sign(s.cfg.ReportTokenSecret, reporttoken.Payload{
		UserID:      userID,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
	}, s.cfg.ReportTokenTTL)
	baseURL := s.cfg.ReportServeBaseURL
	if baseURL == "" {
		baseURL = "http://localhost:8001/api/reports/serve"
	}
	reportURL := baseURL + "?token=" + url.QueryEscape(tok)
	resp := map[string]string{"report_url": reportURL}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("%s [reports] encode error: %v", logPrefix, err)
	}
}

func (s *Server) handleServeReport(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}
	payload, err := reporttoken.Verify(s.cfg.ReportTokenSecret, token)
	if err != nil {
		log.Printf("%s [serve] token verify: %v", logPrefix, err)
		http.Error(w, "invalid or expired token", http.StatusForbidden)
		return
	}
	if payload.UserID != userID {
		log.Printf("%s [serve] user mismatch token_user=%s header_user=%s", logPrefix, payload.UserID, userID)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	key := reportS3Key(payload.UserID, payload.PeriodStart, payload.PeriodEnd)
	body, err := s.storage.Get(r.Context(), key)
	if err != nil {
		log.Printf("%s [serve] S3 Get key=%s: %v", logPrefix, key, err)
		http.Error(w, "report not found", http.StatusNotFound)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=300")
	io.Copy(w, body)
}
