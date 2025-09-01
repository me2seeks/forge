package storage

import (
	"context"
	"io"
	"time"
)

//go:generate  mockgen -destination ../../../internal/mock/infra/contract/storage/storage_mock.go -package mock -source storage.go Factory
type Storage interface {
	PutObject(ctx context.Context, objectKey string, content []byte, opts ...PutOptFn) error
	PutObjectWithReader(ctx context.Context, objectKey string, content io.Reader, opts ...PutOptFn) error
	GetObject(ctx context.Context, objectKey string) ([]byte, error)
	DeleteObject(ctx context.Context, objectKey string) error
	GetObjectUrl(ctx context.Context, objectKey string, opts ...GetOptFn) (string, error)
	// ListObjects returns all objects with the specified prefix.
	// It may return a large number of objects, consider using ListObjectsPaginated for better performance.
	ListObjects(ctx context.Context, prefix string) ([]*FileInfo, error)

	// ListObjectsPaginated returns objects with pagination support.
	// Use this method when dealing with large number of objects.
	ListObjectsPaginated(ctx context.Context, input *ListObjectsPaginatedInput) (*ListObjectsPaginatedOutput, error)
}

type SecurityToken struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	SessionToken    string `json:"session_token"`
	ExpiredTime     string `json:"expired_time"`
	CurrentTime     string `json:"current_time"`
}

type ListObjectsPaginatedInput struct {
	Prefix   string
	PageSize int
	Cursor   string
}

type ListObjectsPaginatedOutput struct {
	Files  []*FileInfo
	Cursor string
	// false: All results have been returned
	// true: There are more results to return
	IsTruncated bool
}
type FileInfo struct {
	Key          string
	LastModified time.Time
	ETag         string
	Size         int64
}
