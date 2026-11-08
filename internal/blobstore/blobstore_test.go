package blobstore

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
)

// Runs against a real bucket when PURSUIT_S3_BUCKET is set, e.g. a local
// MinIO with PURSUIT_S3_ENDPOINT=http://127.0.0.1:9000 and PURSUIT_S3_PATH_STYLE=true.
func TestS3RoundTrip(t *testing.T) {
	s, err := FromEnv(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Skip("PURSUIT_S3_BUCKET is unset")
	}
	ctx := context.Background()
	if ok, err := s.client.BucketExists(ctx, s.bucket); err != nil {
		t.Fatal(err)
	} else if !ok {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	body := []byte("%PDF-1.7 not really")
	if err := s.Put(ctx, "test/roundtrip.pdf", "application/pdf", body); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "test/roundtrip.pdf")
	if err != nil || string(got) != string(body) {
		t.Fatalf("get %q %v", got, err)
	}
	if err := s.Delete(ctx, "test/roundtrip.pdf"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, "test/roundtrip.pdf"); err == nil {
		t.Fatal("deleted object is still there")
	}
}

func TestFromEnvUnset(t *testing.T) {
	if s, err := FromEnv(func(string) string { return "" }); s != nil || err != nil {
		t.Fatalf("want nil, nil; got %v %v", s, err)
	}
}

// A missing bucket is the common setup mistake: say so, in words that name the fix.
func TestMissingBucketIsExplained(t *testing.T) {
	s, err := FromEnv(os.Getenv)
	if err != nil || s == nil {
		t.Skip("PURSUIT_S3_BUCKET is unset")
	}
	s.bucket = "pursuit-no-such-bucket"
	err = s.Put(context.Background(), "x", "text/plain", []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("want a missing-bucket explanation, got %v", err)
	}
}
