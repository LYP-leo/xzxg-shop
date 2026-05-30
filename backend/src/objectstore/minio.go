package objectstore

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOStore struct {
	client *minio.Client
	bucket string
}

type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
}

func NewMinIOFromConfig(values map[string]string) (*MinIOStore, error) {
	endpoint := configString(values, "minio.endpoint", "127.0.0.1:9000")
	accessKey := configString(values, "minio.access_key", "minioadmin")
	secretKey := configString(values, "minio.secret_key", "minioadmin")
	bucket := configString(values, "minio.bucket", "xzxg-shop-assets")
	useSSL := configBool(values, "minio.use_ssl", false)
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	return &MinIOStore{client: client, bucket: bucket}, nil
}

func (s *MinIOStore) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check minio bucket: %w", err)
	}
	if exists {
		return nil
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("create minio bucket: %w", err)
	}
	return nil
}

func (s *MinIOStore) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (ObjectInfo, error) {
	if err := s.EnsureBucket(ctx); err != nil {
		return ObjectInfo{}, err
	}
	info, err := s.client.PutObject(ctx, s.bucket, key, reader, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("put minio object: %w", err)
	}
	return ObjectInfo{Key: info.Key, Size: info.Size, ContentType: contentType}, nil
}

func (s *MinIOStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get minio object: %w", err)
	}
	return object, nil
}

func configString(values map[string]string, key string, fallback string) string {
	value := strings.TrimSpace(values[key])
	if value == "" {
		return fallback
	}
	return value
}

func configBool(values map[string]string, key string, fallback bool) bool {
	value := strings.TrimSpace(values[key])
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
