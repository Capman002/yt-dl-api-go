// Package r2 provides Cloudflare R2 storage operations.
package r2

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Config holds configuration for R2 client.
type Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	PublicURL       string
}

// Client provides operations for Cloudflare R2 storage.
type Client struct {
	s3Client   *s3.Client
	bucketName string
	publicURL  string
}

// NewClient creates a new R2 client.
func NewClient(ctx context.Context, cfg *Config) (*Client, error) {
	// Validate config
	if cfg.AccountID == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" || cfg.BucketName == "" {
		return nil, fmt.Errorf("incomplete R2 configuration")
	}

	// R2 endpoint
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)

	// Create AWS config with static credentials
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with R2 endpoint
	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	slog.Info("R2 client initialized",
		"bucket", cfg.BucketName,
		"endpoint", endpoint,
	)

	return &Client{
		s3Client:   s3Client,
		bucketName: cfg.BucketName,
		publicURL:  cfg.PublicURL,
	}, nil
}

// Upload uploads a file to R2.
func (c *Client) Upload(ctx context.Context, filePath, key string) error {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info for content type
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Determine content type based on extension
	contentType := getContentType(filePath)

	// Upload to R2
	_, err = c.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucketName),
		Key:           aws.String(key),
		Body:          file,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(fileInfo.Size()),
	})

	if err != nil {
		return fmt.Errorf("failed to upload to R2: %w", err)
	}

	slog.Info("File uploaded to R2",
		"key", key,
		"size", fileInfo.Size(),
		"content_type", contentType,
	)

	return nil
}

// GeneratePresignedURL generates a presigned URL for downloading a file.
func (c *Client) GeneratePresignedURL(ctx context.Context, key string, expiryMinutes int) (string, error) {
	presignClient := s3.NewPresignClient(c.s3Client)

	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = time.Duration(expiryMinutes) * time.Minute
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	slog.Debug("Generated presigned URL",
		"key", key,
		"expires_in", expiryMinutes,
	)

	return request.URL, nil
}

// Delete deletes a file from R2.
func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete from R2: %w", err)
	}

	slog.Debug("File deleted from R2", "key", key)

	return nil
}

// ListObjects lists objects in the bucket.
func (c *Client) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	output, err := c.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucketName),
		Prefix: aws.String(prefix),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	keys := make([]string, 0, len(output.Contents))
	for _, obj := range output.Contents {
		if obj.Key != nil {
			keys = append(keys, *obj.Key)
		}
	}

	return keys, nil
}

// ListOlderThan returns keys of objects older than the specified age.
func (c *Client) ListOlderThan(ctx context.Context, age time.Duration) ([]string, error) {
	output, err := c.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucketName),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	threshold := time.Now().Add(-age)
	var oldKeys []string

	for _, obj := range output.Contents {
		if obj.Key != nil && obj.LastModified != nil {
			if obj.LastModified.Before(threshold) {
				oldKeys = append(oldKeys, *obj.Key)
			}
		}
	}

	return oldKeys, nil
}

// DeleteOlderThan deletes objects older than the specified age.
func (c *Client) DeleteOlderThan(ctx context.Context, age time.Duration) (int, error) {
	keys, err := c.ListOlderThan(ctx, age)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, key := range keys {
		if err := c.Delete(ctx, key); err != nil {
			slog.Warn("Failed to delete old file",
				"key", key,
				"error", err,
			)
			continue
		}
		deleted++
	}

	if deleted > 0 {
		slog.Info("Deleted old files from R2",
			"count", deleted,
			"age", age,
		)
	}

	return deleted, nil
}

// getContentType returns the MIME type based on file extension.
func getContentType(filePath string) string {
	ext := filepath.Ext(filePath)
	switch ext {
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	case ".m4a":
		return "audio/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".flac":
		return "audio/flac"
	case ".wav":
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}
