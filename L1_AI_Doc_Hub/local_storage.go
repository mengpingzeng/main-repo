package a4md

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// LocalFSStorage 本地文件系统实现（MVP 开发期用）
type LocalFSStorage struct {
	basePath string
}

func NewLocalFSStorage(basePath string) (*LocalFSStorage, error) {
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, err
	}
	return &LocalFSStorage{basePath: basePath}, nil
}

func (l *LocalFSStorage) fullPath(path string) string {
	return filepath.Join(l.basePath, path)
}

func (l *LocalFSStorage) Put(ctx context.Context, path string, data io.Reader) error {
	full := l.fullPath(path)
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		return err
	}
	f, err := os.Create(full)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, data)
	return err
}

func (l *LocalFSStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	return os.Open(l.fullPath(path))
}

func (l *LocalFSStorage) Delete(ctx context.Context, path string) error {
	return os.Remove(l.fullPath(path))
}

func (l *LocalFSStorage) List(ctx context.Context, prefix string) ([]string, error) {
	var paths []string
	err := filepath.Walk(l.fullPath(prefix), func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(l.basePath, p)
			paths = append(paths, rel)
		}
		return nil
	})
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	}
	return paths, err
}

func (l *LocalFSStorage) Exists(ctx context.Context, path string) (bool, error) {
	_, err := os.Stat(l.fullPath(path))
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
