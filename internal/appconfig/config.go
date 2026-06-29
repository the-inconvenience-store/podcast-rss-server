package appconfig

import (
	"fmt"
	"os"
	"strings"

	"github.com/samstevens/podcast-rss/internal/storage"
)

type Config struct {
	APIKeys       []string
	S3            storage.S3Config
	PublicBaseURL string
	DatabasePath  string
	ListenAddr    string
	DefaultShowID string
}

func Load() (Config, error) {
	cfg := Config{
		APIKeys:       splitCSV(os.Getenv("API_KEYS")),
		PublicBaseURL: os.Getenv("PUBLIC_BASE_URL"),
		DatabasePath:  os.Getenv("DATABASE_PATH"),
		ListenAddr:    getenv("LISTEN_ADDR", ":8080"),
		DefaultShowID: os.Getenv("DEFAULT_SHOW_ID"),
		S3: storage.S3Config{
			Endpoint:        os.Getenv("S3_ENDPOINT"),
			Region:          os.Getenv("S3_REGION"),
			Bucket:          os.Getenv("S3_BUCKET"),
			AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
		},
	}
	if len(cfg.APIKeys) == 0 {
		return Config{}, fmt.Errorf("API_KEYS is required")
	}
	if cfg.PublicBaseURL == "" {
		return Config{}, fmt.Errorf("PUBLIC_BASE_URL is required")
	}
	if cfg.DatabasePath == "" {
		return Config{}, fmt.Errorf("DATABASE_PATH is required")
	}
	if cfg.S3.Endpoint == "" || cfg.S3.Region == "" || cfg.S3.Bucket == "" || cfg.S3.AccessKeyID == "" || cfg.S3.SecretAccessKey == "" {
		return Config{}, fmt.Errorf("complete S3 configuration is required")
	}
	return cfg, nil
}

func splitCSV(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
