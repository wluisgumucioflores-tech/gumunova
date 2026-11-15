// Package config reads the process configuration from environment variables.
// Secrets live only in the environment (or a secret manager), never in the repository.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string

	ObjectStorageEndpoint string
	// ObjectStoragePublicEndpoint is where browsers download signed image URLs; empty means
	// ObjectStorageEndpoint. Only needed when the API reaches the storage by an internal name.
	ObjectStoragePublicEndpoint string
	ObjectStorageRegion         string
	ObjectStorageBucket         string
	ObjectStorageAccessKey      string
	ObjectStorageSecretKey      string
	// AutoCreateBucket creates the bucket on start. Only meant for local development.
	AutoCreateBucket bool
}

// Load reads and validates the configuration, reporting every missing variable at once.
func Load() (Config, error) {
	config := Config{
		HTTPAddr:                    valueOr("HTTP_ADDR", "127.0.0.1:8080"),
		DatabaseURL:                 os.Getenv("DATABASE_URL"),
		ObjectStorageEndpoint:       os.Getenv("OBJECT_STORAGE_ENDPOINT"),
		ObjectStoragePublicEndpoint: os.Getenv("OBJECT_STORAGE_PUBLIC_ENDPOINT"),
		ObjectStorageRegion:         valueOr("OBJECT_STORAGE_REGION", "us-east-1"),
		ObjectStorageBucket:         os.Getenv("OBJECT_STORAGE_BUCKET"),
		ObjectStorageAccessKey:      os.Getenv("OBJECT_STORAGE_ACCESS_KEY"),
		ObjectStorageSecretKey:      os.Getenv("OBJECT_STORAGE_SECRET_KEY"),
	}

	var problems []string
	if autoCreate := os.Getenv("OBJECT_STORAGE_AUTO_CREATE_BUCKET"); autoCreate != "" {
		parsed, err := strconv.ParseBool(autoCreate)
		if err != nil {
			problems = append(problems, "OBJECT_STORAGE_AUTO_CREATE_BUCKET must be true or false")
		}
		config.AutoCreateBucket = parsed
	}

	required := map[string]string{
		"DATABASE_URL":              config.DatabaseURL,
		"OBJECT_STORAGE_ENDPOINT":   config.ObjectStorageEndpoint,
		"OBJECT_STORAGE_BUCKET":     config.ObjectStorageBucket,
		"OBJECT_STORAGE_ACCESS_KEY": config.ObjectStorageAccessKey,
		"OBJECT_STORAGE_SECRET_KEY": config.ObjectStorageSecretKey,
	}
	for _, name := range []string{
		"DATABASE_URL", "OBJECT_STORAGE_ENDPOINT", "OBJECT_STORAGE_BUCKET",
		"OBJECT_STORAGE_ACCESS_KEY", "OBJECT_STORAGE_SECRET_KEY",
	} {
		if required[name] == "" {
			problems = append(problems, name+" is required")
		}
	}

	if len(problems) > 0 {
		return Config{}, errors.New("invalid configuration: " + strings.Join(problems, "; "))
	}
	return config, nil
}

// LoadDatabaseURL reads only DATABASE_URL, for commands that do not need the rest.
func LoadDatabaseURL() (string, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return "", fmt.Errorf("invalid configuration: DATABASE_URL is required")
	}
	return databaseURL, nil
}

func valueOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
