package oss

import "context"

type ObjectStore interface {
	Put(ctx context.Context, path string, data []byte) error
	Get(ctx context.Context, path string) ([]byte, error)
	Exists(ctx context.Context, path string) (bool, error)
	Delete(ctx context.Context, path string) error
}
