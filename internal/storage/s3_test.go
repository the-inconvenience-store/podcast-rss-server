package storage_test

import (
	"testing"

	"github.com/samstevens/podcast-rss/internal/storage"
)

func TestNewS3StorageValidatesRequiredConfig(t *testing.T) {
	for _, cfg := range []storage.S3Config{
		{},
		{Endpoint: "http://localhost:3900", Region: "garage", Bucket: "podcasts", AccessKeyID: "key"},
		{Endpoint: "http://localhost:3900", Region: "garage", Bucket: "podcasts", SecretAccessKey: "secret"},
		{Endpoint: "http://localhost:3900", Region: "garage", AccessKeyID: "key", SecretAccessKey: "secret"},
	} {
		if _, err := storage.NewS3Storage(cfg); err == nil {
			t.Fatalf("NewS3Storage(%+v) returned nil error", cfg)
		}
	}
}
