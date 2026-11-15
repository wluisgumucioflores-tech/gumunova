package ports

import (
	"context"
	"io"
	"time"
)

// ImageStorage stores private image objects (Cloudflare R2 or any S3-compatible service).
type ImageStorage interface {
	Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	// Delete removes an object; deleting a missing object is not an error.
	Delete(ctx context.Context, key string) error
	// SignedURL returns a short-lived URL that allows reading the object.
	SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}
