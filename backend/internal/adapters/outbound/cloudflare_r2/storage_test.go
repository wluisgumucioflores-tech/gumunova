package cloudflare_r2_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"gumunova/backend/internal/adapters/outbound/cloudflare_r2"
)

// newStorage needs an S3-compatible endpoint (the local SeaweedFS container).
// The tests are skipped when TEST_OBJECT_STORAGE_ENDPOINT is not set.
func newStorage(t *testing.T) *cloudflare_r2.Storage {
	t.Helper()
	endpoint := os.Getenv("TEST_OBJECT_STORAGE_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_OBJECT_STORAGE_ENDPOINT is not set; skipping object storage integration test")
	}
	storage := cloudflare_r2.New(cloudflare_r2.Config{
		Endpoint:  endpoint,
		Region:    "us-east-1",
		Bucket:    "catalog-images-test",
		AccessKey: os.Getenv("OBJECT_STORAGE_ACCESS_KEY"),
		SecretKey: os.Getenv("OBJECT_STORAGE_SECRET_KEY"),
	})
	if err := storage.EnsureBucket(context.Background()); err != nil {
		t.Fatalf("ensure bucket: %v", err)
	}
	return storage
}

func TestPutSignedURLAndDelete(t *testing.T) {
	storage := newStorage(t)
	ctx := context.Background()
	key := fmt.Sprintf("products/test/%d.png", time.Now().UnixNano())
	content := []byte("\x89PNG\r\n\x1a\nfake-image-content")

	if err := storage.Put(ctx, key, "image/png", bytes.NewReader(content), int64(len(content))); err != nil {
		t.Fatalf("put: %v", err)
	}

	url, err := storage.SignedURL(ctx, key, time.Minute)
	if err != nil {
		t.Fatalf("signed url: %v", err)
	}
	if !strings.Contains(url, "X-Amz-Signature") {
		t.Errorf("url is not signed: %s", url)
	}

	response, err := http.Get(url)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || !bytes.Equal(body, content) {
		t.Fatalf("download status %d, body %q", response.StatusCode, body)
	}
	if got := response.Header.Get("Content-Type"); got != "image/png" {
		t.Errorf("content type = %q, want image/png", got)
	}

	// An unsigned request must be rejected: the bucket is private.
	unsigned := url[:strings.Index(url, "?")]
	anonymous, err := http.Get(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	anonymous.Body.Close()
	if anonymous.StatusCode == http.StatusOK {
		t.Error("anonymous read of a private object must not succeed")
	}

	if err := storage.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := storage.Delete(ctx, key); err != nil {
		t.Errorf("deleting a missing object must not fail: %v", err)
	}
	gone, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	gone.Body.Close()
	if gone.StatusCode == http.StatusOK {
		t.Error("deleted object must not be readable")
	}
}

func TestEnsureBucketIsIdempotent(t *testing.T) {
	storage := newStorage(t)
	if err := storage.EnsureBucket(context.Background()); err != nil {
		t.Errorf("second call: %v", err)
	}
}
