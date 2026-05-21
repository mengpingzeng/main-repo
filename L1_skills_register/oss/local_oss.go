package oss

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type LocalOSS struct {
	BaseDir string
}

func NewLocalOSS(baseDir string) (*LocalOSS, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create base dir: %w", err)
	}
	return &LocalOSS{BaseDir: baseDir}, nil
}

func (l *LocalOSS) Put(ctx context.Context, path string, data []byte) error {
	fullPath := filepath.Join(l.BaseDir, path)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	return os.WriteFile(fullPath, data, 0644)
}

func (l *LocalOSS) Get(ctx context.Context, path string) ([]byte, error) {
	fullPath := filepath.Join(l.BaseDir, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, err
	}
	return data, nil
}

func (l *LocalOSS) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := filepath.Join(l.BaseDir, path)
	_, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func (l *LocalOSS) Delete(ctx context.Context, path string) error {
	fullPath := filepath.Join(l.BaseDir, path)
	return os.Remove(fullPath)
}
