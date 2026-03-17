package services

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/inventory-back/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var allowedMIMETypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

type MinIOService struct {
	client    *minio.Client
	bucket    string
	maxBytes  int64
	publicURL *url.URL // optional: rewrite presigned URL host for browser access
}

func NewMinIOService(cfg *config.Config) (*MinIOService, error) {
	client, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	var publicURL *url.URL
	if cfg.MinIOPublicURL != "" {
		publicURL, err = url.Parse(cfg.MinIOPublicURL)
		if err != nil {
			return nil, fmt.Errorf("invalid MINIO_PUBLIC_URL: %w", err)
		}
	}

	svc := &MinIOService{
		client:    client,
		bucket:    cfg.MinIOBucket,
		maxBytes:  cfg.MinIOMaxSizeMB * 1024 * 1024,
		publicURL: publicURL,
	}
	if err := svc.ensureBucket(); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *MinIOService) ensureBucket() error {
	ctx := context.Background()
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("bucket check: %w", err)
	}
	if !exists {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
		// Set read-only policy for product images prefix
		policy := fmt.Sprintf(`{
			"Version":"2012-10-17",
			"Statement":[{
				"Effect":"Allow",
				"Principal":{"AWS":["*"]},
				"Action":["s3:GetObject"],
				"Resource":["arn:aws:s3:::%s/products/*"]
			}]
		}`, s.bucket)
		if err := s.client.SetBucketPolicy(ctx, s.bucket, policy); err != nil {
			slog.Warn("could not set bucket policy", slog.String("error", err.Error()))
		}
		slog.Info("minio bucket created", slog.String("bucket", s.bucket))
	}
	return nil
}

// Ping verifies the MinIO connection is healthy.
func (s *MinIOService) Ping() bool {
	_, err := s.client.BucketExists(context.Background(), s.bucket)
	return err == nil
}

// UploadProductImage validates and uploads a product image, returning the object key.
func (s *MinIOService) UploadProductImage(file multipart.File, header *multipart.FileHeader) (string, error) {
	if header.Size > s.maxBytes {
		return "", fmt.Errorf("file too large: maximum %d MB", s.maxBytes/1024/1024)
	}

	// Validate MIME type from Content-Type header
	ct := header.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(ct)
	ext, ok := allowedMIMETypes[mediaType]
	if !ok {
		return "", fmt.Errorf("unsupported image type %q; allowed: jpeg, png, webp", ct)
	}

	// Also sniff first 512 bytes as secondary check
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	sniffed := sniffMIME(buf[:n])
	if sniffed != "" && !strings.HasPrefix(sniffed, "image/") {
		return "", fmt.Errorf("file content does not appear to be an image")
	}
	// Reset reader
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("could not reset file reader: %w", err)
	}

	objectKey := fmt.Sprintf("products/%s%s", uuid.New().String(), ext)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := s.client.PutObject(ctx, s.bucket, objectKey, file, header.Size,
		minio.PutObjectOptions{ContentType: mediaType},
	)
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}
	return objectKey, nil
}

// GetPresignedURL returns a pre-signed GET URL for an object.
// If MINIO_PUBLIC_URL is configured, the scheme and host of the URL are
// replaced so browsers can reach MinIO through its public address.
func (s *MinIOService) GetPresignedURL(objectKey string, expiry time.Duration) (string, error) {
	if objectKey == "" {
		return "", nil
	}
	u, err := s.client.PresignedGetObject(
		context.Background(), s.bucket, objectKey, expiry, url.Values{},
	)
	if err != nil {
		return "", fmt.Errorf("presign: %w", err)
	}
	if s.publicURL != nil {
		u.Scheme = s.publicURL.Scheme
		u.Host = s.publicURL.Host
	}
	return u.String(), nil
}

// DeleteObject removes an object, logging errors rather than returning them
// since it's typically called as a cleanup step.
func (s *MinIOService) DeleteObject(objectKey string) {
	if objectKey == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
		slog.Error("failed to delete object from MinIO",
			slog.String("key", objectKey),
			slog.String("error", err.Error()),
		)
	}
}

// sniffMIME does a basic magic-byte check on the first bytes of a file.
func sniffMIME(data []byte) string {
	if len(data) < 4 {
		return ""
	}
	switch {
	case data[0] == 0xFF && data[1] == 0xD8:
		return "image/jpeg"
	case data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47:
		return "image/png"
	case string(data[:4]) == "RIFF" && len(data) >= 12 && string(data[8:12]) == "WEBP":
		return "image/webp"
	}
	return ""
}
