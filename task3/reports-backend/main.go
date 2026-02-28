package main

import (
	"log"
	"os"
	"strings"

	"reports-backend/internal/config"
	"reports-backend/internal/server"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[reports-backend] config: %v", err)
	}
	// Логируем DSN без пароля
	dsnForLog := cfg.OLAPDSN
	if idx := strings.Index(dsnForLog, "@"); idx > 0 {
		dsnForLog = "***@" + dsnForLog[idx+1:]
	}
	log.Printf("[reports-backend] config: PORT=%s OLAP_DSN=%s", cfg.Port, dsnForLog)

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("[reports-backend] server: %v", err)
	}
	addr := ":" + cfg.Port
	log.Printf("[reports-backend] listening on %s", addr)
	if err := srv.ListenAndServe(addr); err != nil {
		log.Fatalf("[reports-backend] serve: %v", err)
	}
	os.Exit(0)
}
