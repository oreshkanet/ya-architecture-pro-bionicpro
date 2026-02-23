package main

import (
	"log"
	"os"

	"reports-backend/internal/config"
	"reports-backend/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("server: %v", err)
	}
	addr := ":" + cfg.Port
	log.Printf("reports-backend listening on %s", addr)
	if err := srv.ListenAndServe(addr); err != nil {
		log.Fatalf("serve: %v", err)
	}
	os.Exit(0)
}
