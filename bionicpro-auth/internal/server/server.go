package server

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"bionicpro-auth/internal/config"
	"bionicpro-auth/internal/keycloak"
	"bionicpro-auth/internal/session"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

type Server struct {
	cfg      *config.Config
	store    session.Store
	keycloak *keycloak.Client
}

func New(cfg *config.Config) (*Server, error) {
	kc := keycloak.NewClient(cfg.KeycloakURL, cfg.KeycloakRealm, cfg.KeycloakClientID, cfg.KeycloakSecret)
	return &Server{
		cfg:      cfg,
		store:    session.NewInMemoryStore(),
		keycloak: kc,
	}, nil
}

type responseLogger struct {
	http.ResponseWriter
	status  int
	written int64
}

func (rl *responseLogger) WriteHeader(code int) {
	rl.status = code
	rl.ResponseWriter.WriteHeader(code)
}

func (rl *responseLogger) Write(b []byte) (int, error) {
	n, err := rl.ResponseWriter.Write(b)
	rl.written += int64(n)
	return n, err
}

func requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseLogger{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		dur := time.Since(start)
		log.Printf("[in] %s %s from %s - %d %dB in %v", r.Method, r.URL.RequestURI(), r.RemoteAddr, wrapped.status, wrapped.written, dur)
	})
}

func (s *Server) ListenAndServe(addr string) error {
	r := chi.NewRouter()
	r.Use(requestLoggingMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(s.corsMiddleware())

	r.Get("/login", s.handleLogin)
	r.Get("/auth/callback", s.handleCallback)
	r.Get("/logout", s.handleLogout)
	r.Get("/session/check", s.handleSessionCheck)

	// защищённый эндпоинт — проверка сессии с ротацией
	r.Group(func(r chi.Router) {
		r.Use(s.requireSession)
		r.Get("/session/validate", s.handleSessionValidate)
		r.Get("/api/reports", s.handleReportsProxy)
	})

	return http.ListenAndServe(addr, r)
}

func (s *Server) corsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				origin = s.cfg.FrontendURL
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Cookie")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	baseURL := s.cfg.KeycloakPublicURL
	if baseURL == "" {
		baseURL = s.cfg.KeycloakURL
	}
	authURL := baseURL + "/realms/" + s.cfg.KeycloakRealm + "/protocol/openid-connect/auth"
	redirectURI := s.cfg.AuthCallbackURL
	state := uuid.New().String()

	u, _ := url.Parse(authURL)
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", s.cfg.KeycloakClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", "openid offline_access")
	q.Set("state", state)
	u.RawQuery = q.Encode()

	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, s.cfg.FrontendURL+"?error=no_code", http.StatusFound)
		return
	}
	redirectURI := s.cfg.AuthCallbackURL

	tokens, err := s.keycloak.ExchangeCode(r.Context(), code, redirectURI)
	if err != nil {
		log.Printf("[auth] token exchange failed: %v", err)
		http.Redirect(w, r, s.cfg.FrontendURL+"?error=token_exchange", http.StatusFound)
		return
	}

	encryptedRefresh, err := session.EncryptRefreshToken(tokens.RefreshToken, s.cfg.EncryptionKey)
	if err != nil {
		http.Redirect(w, r, s.cfg.FrontendURL+"?error=encrypt", http.StatusFound)
		return
	}

	sessionID := uuid.New().String()
	ttl := time.Duration(s.cfg.SessionMaxAge) * time.Second
	expiresAt := time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)

	err = s.store.Set(r.Context(), sessionID, &session.SessionData{
		AccessToken:  tokens.AccessToken,
		RefreshToken: encryptedRefresh,
		ExpiresAt:    expiresAt,
	}, ttl)
	if err != nil {
		http.Redirect(w, r, s.cfg.FrontendURL+"?error=store", http.StatusFound)
		return
	}

	s.setSessionCookie(w, sessionID)
	http.Redirect(w, r, s.cfg.FrontendURL, http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(s.cfg.SessionCookieName)
	if err == nil && cookie.Value != "" {
		_ = s.store.Delete(r.Context(), cookie.Value)
	}
	s.clearSessionCookie(w)
	http.Redirect(w, r, s.cfg.FrontendURL, http.StatusFound)
}

func (s *Server) handleSessionCheck(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(s.cfg.SessionCookieName)
	if err != nil || cookie.Value == "" {
		log.Printf("[session] cookie not defined")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	data, err := s.store.Get(r.Context(), cookie.Value)
	if err != nil || data == nil {
		log.Printf("[session] token not defined")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReportsProxy(w http.ResponseWriter, r *http.Request) {
	token, _ := r.Context().Value("access_token").(string)
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	targetURL := s.cfg.ReportsAPIURL + "/reports"
	log.Printf("[out] GET %s (reports API)", targetURL)
	req, err := http.NewRequestWithContext(r.Context(), "GET", targetURL, nil)
	if err != nil {
		log.Printf("[out] GET %s - error: %v", targetURL, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[out] GET %s - error: %v", targetURL, err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	log.Printf("[out] GET %s - %d", targetURL, resp.StatusCode)
	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (s *Server) handleSessionValidate(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(s.cfg.SessionCookieName)
	if cookie == nil || cookie.Value == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	newSessionID, _, err := s.store.Rotate(r.Context(), cookie.Value)
	if err != nil || newSessionID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	s.setSessionCookie(w, newSessionID)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(s.cfg.SessionCookieName)
		if err != nil || cookie.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		data, err := s.store.Get(r.Context(), cookie.Value)
		if err != nil || data == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		accessToken, err := s.ensureValidAccessToken(r.Context(), data)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// Ротация сессии для предотвращения session fixation
		newSessionID, _, err := s.store.Rotate(r.Context(), cookie.Value)
		if err == nil && newSessionID != "" {
			s.setSessionCookie(w, newSessionID)
		}
		r = r.WithContext(context.WithValue(r.Context(), "access_token", accessToken))
		next.ServeHTTP(w, r)
	})
}

func (s *Server) ensureValidAccessToken(ctx context.Context, data *session.SessionData) (string, error) {
	if !data.ExpiresAt.IsZero() && time.Now().Before(data.ExpiresAt) {
		return data.AccessToken, nil
	}
	refreshToken, err := session.DecryptRefreshToken(data.RefreshToken, s.cfg.EncryptionKey)
	if err != nil {
		return "", err
	}
	tokens, err := s.keycloak.RefreshToken(ctx, refreshToken)
	if err != nil {
		return "", err
	}
	data.AccessToken = tokens.AccessToken
	data.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	if tokens.RefreshToken != "" {
		encryptedRefresh, err := session.EncryptRefreshToken(tokens.RefreshToken, s.cfg.EncryptionKey)
		if err != nil {
			return "", err
		}
		data.RefreshToken = encryptedRefresh
	}
	return data.AccessToken, nil
}

func (s *Server) setSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   s.cfg.SessionMaxAge,
		HttpOnly: true,
		Secure:   s.cfg.SecureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cfg.SecureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) getAuthCallbackURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost:8001"
	}
	return scheme + "://" + host + "/auth/callback"
}
