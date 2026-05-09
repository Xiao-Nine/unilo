package storage

import (
	"context"
	"io"
)

type ObjectInfo struct {
	Size        int64
	ContentType string
}

type Client interface {
	EnsureBucket(ctx context.Context) error
	Put(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error
	Get(ctx context.Context, objectKey string) (io.ReadCloser, ObjectInfo, error)
	Delete(ctx context.Context, objectKey string) error
}
