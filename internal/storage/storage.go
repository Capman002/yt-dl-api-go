// Package storage provides file storage implementations.
package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2 implements Storage using Cloudflare R2.
type R2 struct {
	client    *s3.Client
	bucket    string
	publicURL string
}

// NewR2 creates a new R2 storage client.
func NewR2(ctx context.Context, accountID, accessKeyID, secretAccessKey, bucket, publicURL string) (*R2, error) {
	if accountID == "" || accessKeyID == "" || secretAccessKey == "" {
		return nil, fmt.Errorf("R2 credentials not configured")
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load R2 config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	return &R2{client: client, bucket: bucket, publicURL: publicURL}, nil
}

// Upload uploads a file to R2 and returns the public URL.
func (r *R2) Upload(ctx context.Context, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Generate unique key
	key := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(filePath))

	_, err = r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(detectContentType(filePath)),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to R2: %w", err)
	}

	// Build public URL
	if r.publicURL != "" {
		return fmt.Sprintf("%s/%s", r.publicURL, key), nil
	}
	return fmt.Sprintf("https://%s.r2.dev/%s", r.bucket, key), nil
}

// Cleanup removes a local file.
func (r *R2) Cleanup(filePath string) error {
	return os.Remove(filePath)
}

// Local implements Storage using local filesystem.
type Local struct {
	dir string
}

// NewLocal creates a new local storage.
func NewLocal(dir string) *Local {
	os.MkdirAll(dir, 0755)
	return &Local{dir: dir}
}

// Upload copies file and returns a local path (for development).
func (l *Local) Upload(ctx context.Context, filePath string) (string, error) {
	// In local mode, just return the file path as-is
	// In production, you'd want a proper file server
	return filePath, nil
}

// Cleanup does nothing for local storage (file should be served first).
func (l *Local) Cleanup(filePath string) error {
	// Don't delete immediately in local mode - file needs to be downloaded first
	// Cleanup should happen via a separate job or TTL
	return nil
}

// detectContentType returns MIME type based on file extension.
func detectContentType(filePath string) string {
	ext := filepath.Ext(filePath)
	switch ext {
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4"
	default:
		return "application/octet-stream"
	}
}
