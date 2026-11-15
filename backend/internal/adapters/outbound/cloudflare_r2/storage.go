// Package cloudflare_r2 implements image storage over the S3 API.
//
// Cloudflare R2 and the local SeaweedFS container both speak S3, so the same adapter
// serves production and development; only the configuration differs.
package cloudflare_r2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"gumunova/backend/internal/application/ports"
)

// Config describes an S3-compatible endpoint and bucket.
type Config struct {
	Endpoint string // for example https://<account>.r2.cloudflarestorage.com
	// PublicEndpoint is the address browsers use to download signed URLs. Leave it empty when it
	// is the same as Endpoint (Cloudflare R2). It differs when the API reaches the storage through
	// an internal name (a Docker network) that the browser cannot resolve.
	PublicEndpoint string
	Region         string // R2 accepts "auto"; most other S3 services need a real region name
	Bucket         string
	AccessKey      string
	SecretKey      string
}

type Storage struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

var _ ports.ImageStorage = (*Storage)(nil)

func New(config Config) *Storage {
	newClient := func(endpoint string) *s3.Client {
		return s3.New(s3.Options{
			BaseEndpoint: aws.String(endpoint),
			Region:       config.Region,
			Credentials:  credentials.NewStaticCredentialsProvider(config.AccessKey, config.SecretKey, ""),
			// Path-style addressing works for R2 and for local S3 emulators.
			UsePathStyle: true,
			// Only add checksums when an operation requires them; some S3-compatible
			// services reject the optional ones newer SDKs send by default.
			RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
			ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
		})
	}
	client := newClient(config.Endpoint)
	// A signature covers the host, so URLs meant for browsers are signed against the public
	// address. Signing is a local computation: this client never sends a request.
	signer := client
	if config.PublicEndpoint != "" {
		signer = newClient(config.PublicEndpoint)
	}
	return &Storage{client: client, presign: s3.NewPresignClient(signer), bucket: config.Bucket}
}

// EnsureBucket creates the bucket when it does not exist. Intended for local development;
// in production the bucket is provisioned by an administrator.
func (s *Storage) EnsureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err == nil {
		return nil
	}
	var notFound *types.NotFound
	if !errors.As(err, &notFound) {
		var apiError smithy.APIError
		if !errors.As(err, &apiError) || apiError.ErrorCode() != "NotFound" {
			return fmt.Errorf("check bucket: %w", err)
		}
	}
	if _, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(s.bucket)}); err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}
	return nil
}

func (s *Storage) Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}

func (s *Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

func (s *Storage) SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	request, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign get object: %w", err)
	}
	return request.URL, nil
}
