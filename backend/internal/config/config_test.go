package config_test

import (
	"strings"
	"testing"

	"gumunova/backend/internal/config"
)

func setAll(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("OBJECT_STORAGE_ENDPOINT", "http://localhost:8333")
	t.Setenv("OBJECT_STORAGE_BUCKET", "bucket")
	t.Setenv("OBJECT_STORAGE_ACCESS_KEY", "access")
	t.Setenv("OBJECT_STORAGE_SECRET_KEY", "secret")
	// Optional variables start unset so a developer's own environment cannot leak in.
	for _, name := range []string{"HTTP_ADDR", "OBJECT_STORAGE_REGION", "OBJECT_STORAGE_PUBLIC_ENDPOINT", "OBJECT_STORAGE_AUTO_CREATE_BUCKET"} {
		t.Setenv(name, "")
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	setAll(t)
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.HTTPAddr != "127.0.0.1:8080" || loaded.ObjectStorageRegion != "us-east-1" || loaded.AutoCreateBucket {
		t.Errorf("unexpected defaults: %+v", loaded)
	}
}

func TestLoadReadsOptionalPublicEndpoint(t *testing.T) {
	setAll(t)
	t.Setenv("OBJECT_STORAGE_PUBLIC_ENDPOINT", "http://localhost:8333")
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.ObjectStoragePublicEndpoint != "http://localhost:8333" {
		t.Errorf("public endpoint = %q", loaded.ObjectStoragePublicEndpoint)
	}
}

func TestLoadReportsEveryMissingVariable(t *testing.T) {
	for _, name := range []string{
		"DATABASE_URL", "OBJECT_STORAGE_ENDPOINT", "OBJECT_STORAGE_BUCKET",
		"OBJECT_STORAGE_ACCESS_KEY", "OBJECT_STORAGE_SECRET_KEY",
	} {
		t.Setenv(name, "")
	}
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, name := range []string{"DATABASE_URL", "OBJECT_STORAGE_ENDPOINT", "OBJECT_STORAGE_BUCKET", "OBJECT_STORAGE_ACCESS_KEY", "OBJECT_STORAGE_SECRET_KEY"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error does not mention %s: %v", name, err)
		}
	}
}

func TestLoadDoesNotEchoSecretValues(t *testing.T) {
	setAll(t)
	t.Setenv("OBJECT_STORAGE_AUTO_CREATE_BUCKET", "not-a-bool")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "pass") {
		t.Errorf("error leaks configuration values: %v", err)
	}
}

func TestLoadAutoCreateBucket(t *testing.T) {
	setAll(t)
	t.Setenv("OBJECT_STORAGE_AUTO_CREATE_BUCKET", "true")
	loaded, err := config.Load()
	if err != nil || !loaded.AutoCreateBucket {
		t.Errorf("loaded=%+v err=%v", loaded, err)
	}
}
