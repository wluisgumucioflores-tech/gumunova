package cloudflare_r2_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"gumunova/backend/internal/adapters/outbound/cloudflare_r2"
)

// Signing is a local computation, so these tests need no running storage.
func TestSignedURLUsesPublicEndpointWhenConfigured(t *testing.T) {
	storage := cloudflare_r2.New(cloudflare_r2.Config{
		Endpoint:       "http://object-storage:8333",
		PublicEndpoint: "http://localhost:8333",
		Region:         "us-east-1",
		Bucket:         "catalog-images",
		AccessKey:      "access",
		SecretKey:      "secret",
	})
	url, err := storage.SignedURL(context.Background(), "products/a.png", time.Minute)
	if err != nil {
		t.Fatalf("signed url: %v", err)
	}
	if !strings.HasPrefix(url, "http://localhost:8333/catalog-images/products/a.png?") {
		t.Errorf("url must point at the public endpoint, got %s", url)
	}
	if !strings.Contains(url, "X-Amz-Signature") {
		t.Errorf("url is not signed: %s", url)
	}
}

func TestSignedURLUsesEndpointWithoutPublicEndpoint(t *testing.T) {
	storage := cloudflare_r2.New(cloudflare_r2.Config{
		Endpoint:  "http://object-storage:8333",
		Region:    "us-east-1",
		Bucket:    "catalog-images",
		AccessKey: "access",
		SecretKey: "secret",
	})
	url, err := storage.SignedURL(context.Background(), "products/a.png", time.Minute)
	if err != nil {
		t.Fatalf("signed url: %v", err)
	}
	if !strings.HasPrefix(url, "http://object-storage:8333/catalog-images/products/a.png?") {
		t.Errorf("url must point at the endpoint, got %s", url)
	}
}
