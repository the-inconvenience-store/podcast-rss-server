package appconfig_test

import (
	"testing"

	"github.com/samstevens/podcast-rss/internal/appconfig"
)

func TestLoadReadsEnvironmentConfig(t *testing.T) {
	t.Setenv("API_KEYS", "one, two")
	t.Setenv("S3_ENDPOINT", "http://garage:3900")
	t.Setenv("S3_REGION", "garage")
	t.Setenv("S3_BUCKET", "podcasts")
	t.Setenv("S3_ACCESS_KEY_ID", "garage-key")
	t.Setenv("S3_SECRET_ACCESS_KEY", "garage-secret")
	t.Setenv("PUBLIC_BASE_URL", "https://podcasts.example.com")
	t.Setenv("DATABASE_PATH", "/data/podcasts.db")
	t.Setenv("LISTEN_ADDR", ":9090")
	t.Setenv("DEFAULT_SHOW_ID", "show-1")

	cfg, err := appconfig.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(cfg.APIKeys) != 2 || cfg.APIKeys[0] != "one" || cfg.APIKeys[1] != "two" {
		t.Fatalf("APIKeys = %#v", cfg.APIKeys)
	}
	if cfg.S3.Bucket != "podcasts" || cfg.PublicBaseURL != "https://podcasts.example.com" || cfg.ListenAddr != ":9090" || cfg.DefaultShowID != "show-1" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadRejectsMissingRequiredValues(t *testing.T) {
	t.Setenv("API_KEYS", "")
	t.Setenv("PUBLIC_BASE_URL", "https://podcasts.example.com")
	t.Setenv("DATABASE_PATH", "/data/podcasts.db")

	if _, err := appconfig.Load(); err == nil {
		t.Fatal("Load returned nil error")
	}
}
