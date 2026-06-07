package storage_test

import (
	"testing"

	"github.com/JaimeStill/herald/pkg/storage"
)

func TestFinalizeDefaults(t *testing.T) {
	cfg := storage.Config{Endpoint: "localhost:9000", AccessKey: "key", SecretKey: "secret"}
	if err := cfg.Finalize(nil); err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	if cfg.BucketName != "documents" {
		t.Errorf("bucket_name: got %s, want documents", cfg.BucketName)
	}
	if cfg.MaxListSize != 50 {
		t.Errorf("max_list_size: got %d, want 50", cfg.MaxListSize)
	}
	if cfg.UseSSL {
		t.Error("use_ssl: got true, want false")
	}
}

func TestFinalizeEnvOverrides(t *testing.T) {
	t.Setenv("TEST_ENDPOINT", "minio:9000")
	t.Setenv("TEST_ACCESS", "testkey")
	t.Setenv("TEST_SECRET", "testsecret")
	t.Setenv("TEST_BUCKET", "uploads")
	t.Setenv("TEST_SSL", "true")

	env := &storage.Env{
		Endpoint:   "TEST_ENDPOINT",
		AccessKey:  "TEST_ACCESS",
		SecretKey:  "TEST_SECRET",
		BucketName: "TEST_BUCKET",
		UseSSL:     "TEST_SSL",
	}

	cfg := storage.Config{}
	if err := cfg.Finalize(env); err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	if cfg.Endpoint != "minio:9000" {
		t.Errorf("endpoint: got %s, want minio:9000", cfg.Endpoint)
	}
	if cfg.AccessKey != "testkey" {
		t.Errorf("access_key: got %s, want testkey", cfg.AccessKey)
	}
	if cfg.SecretKey != "testsecret" {
		t.Errorf("secret_key: got %s, want testsecret", cfg.SecretKey)
	}
	if cfg.BucketName != "uploads" {
		t.Errorf("bucket_name: got %s, want uploads", cfg.BucketName)
	}
	if !cfg.UseSSL {
		t.Error("use_ssl: got false, want true")
	}
}

func TestFinalizeMaxListSizeCap(t *testing.T) {
	cfg := storage.Config{MaxListSize: 10000}
	if err := cfg.Finalize(nil); err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	if cfg.MaxListSize != storage.MaxListCap {
		t.Errorf("max_list_size: got %d, want %d (capped)", cfg.MaxListSize, storage.MaxListCap)
	}
}

func TestFinalizeMaxListSizeEnvOverride(t *testing.T) {
	t.Setenv("TEST_MAX_LIST", "200")

	env := &storage.Env{
		MaxListSize: "TEST_MAX_LIST",
	}

	cfg := storage.Config{}
	if err := cfg.Finalize(env); err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	if cfg.MaxListSize != 200 {
		t.Errorf("max_list_size: got %d, want 200", cfg.MaxListSize)
	}
}

func TestFinalizeMaxListSizeEnvCapped(t *testing.T) {
	t.Setenv("TEST_MAX_LIST", "99999")

	env := &storage.Env{
		MaxListSize: "TEST_MAX_LIST",
	}

	cfg := storage.Config{}
	if err := cfg.Finalize(env); err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	if cfg.MaxListSize != storage.MaxListCap {
		t.Errorf("max_list_size: got %d, want %d (capped)", cfg.MaxListSize, storage.MaxListCap)
	}
}

func TestMerge(t *testing.T) {
	base := storage.Config{
		BucketName:  "documents",
		Endpoint:    "localhost:9000",
		MaxListSize: 50,
	}

	overlay := storage.Config{
		Endpoint:    "minio:9000",
		MaxListSize: 100,
	}
	base.Merge(&overlay)

	if base.BucketName != "documents" {
		t.Errorf("bucket_name should remain documents, got %s", base.BucketName)
	}
	if base.Endpoint != "minio:9000" {
		t.Errorf("endpoint: got %s, want minio:9000", base.Endpoint)
	}
	if base.MaxListSize != 100 {
		t.Errorf("max_list_size: got %d, want 100", base.MaxListSize)
	}
}

func TestMergeZeroMaxListSizePreservesBase(t *testing.T) {
	base := storage.Config{
		BucketName:  "documents",
		Endpoint:    "localhost:9000",
		MaxListSize: 50,
	}

	overlay := storage.Config{}
	base.Merge(&overlay)

	if base.MaxListSize != 50 {
		t.Errorf("max_list_size: got %d, want 50 (preserved)", base.MaxListSize)
	}
}

func TestMergeUseSSL(t *testing.T) {
	base := storage.Config{BucketName: "documents"}
	overlay := storage.Config{UseSSL: true}

	base.Merge(&overlay)

	if !base.UseSSL {
		t.Error("use_ssl: got false, want true after merge")
	}
}
