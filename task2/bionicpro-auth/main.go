package main

import (
	"log"
	"os"

	"bionicpro-auth/internal/config"
	"bionicpro-auth/internal/server"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[bionicpro-auth] Failed to load config: %v", err)
	}

	log.Printf("[bionicpro-auth] config: PORT=%s KEYCLOAK_URL=%s KEYCLOAK_REALM=%s FRONTEND_URL=%s AUTH_CALLBACK=%s REPORTS_API=%s",
		cfg.Port, cfg.KeycloakURL, cfg.KeycloakRealm, cfg.FrontendURL, cfg.AuthCallbackURL, cfg.ReportsAPIURL)

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("[bionicpro-auth] Failed to create server: %v", err)
	}

	addr := ":" + cfg.Port
	log.Printf("[bionicpro-auth] listening on %s", addr)
	if err := srv.ListenAndServe(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
	os.Exit(0)
}
