// Package blobstore keeps document files outside the database: any S3 API
// (AWS, R2, MinIO, OCI). Without a bucket configured, files stay in the
// database as before.
package blobstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Store puts, gets and deletes whole objects by key.
type Store interface {
	Put(ctx context.Context, key, contentType string, body []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// S3 is a bucket on any S3-compatible service.
type S3 struct {
	client *minio.Client
	bucket string
}

// FromEnv returns the configured bucket, or nil when PURSUIT_S3_BUCKET is
// unset. The endpoint defaults to AWS; credentials fall back to the usual
// AWS_ variables.
func FromEnv(getenv func(string) string) (*S3, error) {
	env := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(getenv(k)); v != "" {
				return v
			}
		}
		return ""
	}
	bucket := env("PURSUIT_S3_BUCKET")
	if bucket == "" {
		return nil, nil
	}
	endpoint := env("PURSUIT_S3_ENDPOINT")
	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
	}
	secure := !strings.HasPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(env("PURSUIT_S3_ACCESS_KEY_ID", "AWS_ACCESS_KEY_ID"), env("PURSUIT_S3_SECRET_ACCESS_KEY", "AWS_SECRET_ACCESS_KEY"), ""),
		Secure: secure,
		Region: env("PURSUIT_S3_REGION", "AWS_REGION"),
	}
	// MinIO and most self-hosted servers only do path-style addressing.
	if env("PURSUIT_S3_PATH_STYLE") == "true" {
		opts.BucketLookup = minio.BucketLookupPath
	}
	c, err := minio.New(endpoint, opts)
	if err != nil {
		return nil, fmt.Errorf("s3: %w", err)
	}
	return &S3{client: c, bucket: bucket}, nil
}

func (s *S3) Put(ctx context.Context, key, contentType string, body []byte) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(body), int64(len(body)), minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(obj)
}

func (s *S3) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

// Memory is a Store for tests.
type Memory struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (m *Memory) Put(_ context.Context, key, _ string, body []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.m == nil {
		m.m = map[string][]byte{}
	}
	m.m[key] = append([]byte(nil), body...)
	return nil
}

func (m *Memory) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.m[key]
	if !ok {
		return nil, fmt.Errorf("blob %q not found", key)
	}
	return b, nil
}

func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, key)
	return nil
}

// Len is how many objects are held.
func (m *Memory) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.m)
}
