package main

import (
	"log"
	"os"

	"bionicpro-auth/internal/config"
	"bionicpro-auth/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	addr := ":" + cfg.Port
	log.Printf("bionicpro-auth listening on %s", addr)
	if err := srv.ListenAndServe(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
	os.Exit(0)
}
