package main

import (
	"log"
	"net/http"

	"github.com/samstevens/podcast-rss/internal/appconfig"
	"github.com/samstevens/podcast-rss/internal/podcast"
	"github.com/samstevens/podcast-rss/internal/server"
	"github.com/samstevens/podcast-rss/internal/storage"
)

func main() {
	cfg, err := appconfig.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	repo, err := podcast.OpenSQLiteRepository(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer repo.Close()

	store, err := storage.NewS3Storage(cfg.S3)
	if err != nil {
		log.Fatalf("configure s3: %v", err)
	}
	handler := server.New(repo, store, server.Config{
		APIKeys:       cfg.APIKeys,
		PublicBaseURL: cfg.PublicBaseURL,
		DefaultShowID: cfg.DefaultShowID,
	})
	log.Printf("listening on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, handler); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
