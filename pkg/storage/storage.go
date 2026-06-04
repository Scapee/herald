// Package storage provides blob storage operations with a MinIO implementation.
package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/JaimeStill/herald/pkg/lifecycle"
)

// MaxListCap is the maximum number of blobs that can be returned in a single list request.
const MaxListCap int32 = 5000

// BlobMeta contains metadata about a single blob in storage.
type BlobMeta struct {
	Name          string    `json:"name"`
	ContentType   string    `json:"content_type"`
	ContentLength int64     `json:"content_length"`
	LastModified  time.Time `json:"last_modified"`
	ETag          string    `json:"etag"`
	CreatedAt     time.Time `json:"created_at"`
}

// BlobList holds a page of blob metadata with an optional continuation marker
// for marker-based pagination.
type BlobList struct {
	Blobs      []BlobMeta `json:"blobs"`
	NextMarker string     `json:"next_marker,omitempty"`
}

// BlobResult bundles blob metadata with the download body stream.
// The caller must close Body when finished reading.
type BlobResult struct {
	BlobMeta
	Body io.ReadCloser `json:"-"`
}

// System manages blob storage operations and lifecycle coordination.
type System interface {
	// Start registers a startup hook that initializes the storage bucket.
	Start(lc *lifecycle.Coordinator) error

	// List returns a page of blob metadata filtered by prefix.
	// Marker is an opaque continuation token from a previous BlobList.
	List(ctx context.Context, prefix string, marker string, maxResults int32) (*BlobList, error)
	// Find returns metadata for a single blob by key.
	// Returns ErrNotFound if the blob does not exist.
	Find(ctx context.Context, key string) (*BlobMeta, error)

	// Upload streams data to a blob at the given key with the specified content type.
	Upload(ctx context.Context, key string, reader io.Reader, contentType string) error
	// Download returns a stream for the blob at the given key. The caller must close the reader.
	// Returns ErrNotFound if the blob does not exist.
	Download(ctx context.Context, key string) (*BlobResult, error)
	// Delete removes the blob at the given key. Returns ErrNotFound if the blob does not exist.
	Delete(ctx context.Context, key string) error
	// Exists reports whether a blob exists at the given key.
	Exists(ctx context.Context, key string) (bool, error)
}

type store struct {
	client *minio.Client
	bucket string
	logger *slog.Logger
}

// New creates a storage system from the given configuration.
// It validates connection parameters and creates the MinIO client
// but does not establish a connection until Start is called.
func New(cfg *Config, logger *slog.Logger) (System, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create storage client: %w", err)
	}

	return &store{
		client: client,
		bucket: cfg.BucketName,
		logger: logger.With("system", "storage"),
	}, nil
}

func (s *store) Start(lc *lifecycle.Coordinator) error {
	s.logger.Info("starting storage system")

	lc.OnStartup(func() {
		ctx := lc.Context()
		exists, err := s.client.BucketExists(ctx, s.bucket)
		if err != nil {
			s.logger.Error("storage bucket check failed", "error", err)
			return
		}

		if !exists {
			if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
				s.logger.Error("storage bucket creation failed", "error", err)
				return
			}
		}

		s.logger.Info("storage bucket ready", "bucket", s.bucket)
	})

	return nil
}

func (s *store) List(
	ctx context.Context,
	prefix string,
	marker string,
	maxResults int32,
) (*BlobList, error) {
	opts := minio.ListObjectsOptions{
		Prefix:     prefix,
		StartAfter: marker,
	}

	blobs := make([]BlobMeta, 0, maxResults)
	var nextMarker string

	for obj := range s.client.ListObjects(ctx, s.bucket, opts) {
		if obj.Err != nil {
			return nil, fmt.Errorf("list blobs: %w", obj.Err)
		}
		if int32(len(blobs)) >= maxResults {
			nextMarker = obj.Key
			break
		}
		blobs = append(blobs, BlobMeta{
			Name:          obj.Key,
			ContentType:   obj.ContentType,
			ContentLength: obj.Size,
			LastModified:  obj.LastModified,
			ETag:          strings.Trim(obj.ETag, `"`),
		})
	}

	return &BlobList{Blobs: blobs, NextMarker: nextMarker}, nil
}

func (s *store) Find(ctx context.Context, key string) (*BlobMeta, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}

	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get blob properties %s: %w", key, err)
	}

	return &BlobMeta{
		Name:          key,
		ContentType:   info.ContentType,
		ContentLength: info.Size,
		LastModified:  info.LastModified,
		ETag:          strings.Trim(info.ETag, `"`),
	}, nil
}

func (s *store) Upload(ctx context.Context, key string, reader io.Reader, contentType string) error {
	if err := validateKey(key); err != nil {
		return err
	}

	_, err := s.client.PutObject(ctx, s.bucket, key, reader, -1, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("upload blob %s: %w", key, err)
	}

	return nil
}

func (s *store) Download(ctx context.Context, key string) (*BlobResult, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}

	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("download blob %s: %w", key, err)
	}

	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("stat blob %s: %w", key, err)
	}

	return &BlobResult{
		BlobMeta: BlobMeta{
			Name:          key,
			ContentType:   info.ContentType,
			ContentLength: info.Size,
		},
		Body: obj,
	}, nil
}

func (s *store) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}

	// Verify existence first so we can return ErrNotFound consistently.
	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("delete blob %s: %w", key, err)
	}

	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete blob %s: %w", key, err)
	}

	return nil
}

func (s *store) Exists(ctx context.Context, key string) (bool, error) {
	if err := validateKey(key); err != nil {
		return false, err
	}

	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("check blob existence %s: %w", key, err)
	}

	return true, nil
}

func isNotFound(err error) bool {
	resp := minio.ToErrorResponse(err)
	return resp.Code == "NoSuchKey" || resp.Code == "NoSuchBucket"
}

func validateKey(key string) error {
	if key == "" {
		return ErrEmptyKey
	}
	if strings.Contains(key, "..") {
		return ErrInvalidKey
	}
	return nil
}

// ParseMaxResults parses a max_results query parameter string into an int32,
// clamping the value at MaxListCap. Returns fallback when s is empty.
func ParseMaxResults(s string, fallback int32) (int32, error) {
	if s == "" {
		return fallback, nil
	}

	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid max_results parameter")
	}

	return min(int32(n), MaxListCap), nil
}
