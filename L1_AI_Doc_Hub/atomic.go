package a4md

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const tmpDir = "/tmp/a4md"

// atomicWrite 原子写入：内存 → OSS，临时文件兜底
func (s *Service) atomicWrite(ctx context.Context, ossPath string, content []byte) error {
	if len(content) > maxFileSize {
		return fmt.Errorf("%w: actual=%d bytes", ErrFileTooLarge, len(content))
	}

	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		return err
	}

	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("md_%s_%d.tmp",
		filepath.Base(ossPath), time.Now().UnixNano()))
	if err := os.WriteFile(tmpFile, content, 0600); err != nil {
		return err
	}

	reader := bytes.NewReader(content)
	if err := s.storage.Put(ctx, ossPath, reader); err != nil {
		return fmt.Errorf("%w: %w", ErrOSSWriteFail, err)
	}

	_ = os.Remove(tmpFile)
	return nil
}

// writeWithRetry OSS 写入重试（指数退避）
func (s *Service) writeWithRetry(ctx context.Context, ossPath string, content []byte) error {
	const (
		maxRetries = 3
		baseDelay  = 500 * time.Millisecond
	)

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := s.atomicWrite(ctx, ossPath, content); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if i < maxRetries-1 {
			delay := baseDelay * (1 << i)
			s.logger.Warn("a4md: OSS write retry", "attempt", i+1, "path", ossPath, "delay", delay, "error", lastErr)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return fmt.Errorf("a4md: OSS write failed after %d retries: %w", maxRetries, lastErr)
}

// writeLargeContent 处理超大 MD 内容的分片写入
func (s *Service) writeLargeContent(ctx context.Context, taskID string, draftVersion int, content []byte) (string, error) {
	if len(content) <= maxFileSize {
		path := buildOSSPath(taskID, draftVersion, 0)
		return path, s.atomicWrite(ctx, path, content)
	}

	var partNo int
	var mainPath string

	for offset := 0; offset < len(content); offset += maxFileSize {
		end := offset + maxFileSize
		if end > len(content) {
			end = len(content)
		}
		chunk := content[offset:end]

		path := buildOSSPath(taskID, draftVersion, partNo)
		if err := s.atomicWrite(ctx, path, chunk); err != nil {
			return "", fmt.Errorf("a4md: write part %d: %w", partNo, err)
		}

		if partNo == 0 {
			mainPath = path
		}
		partNo++
	}

	s.logger.Warn("a4md: content exceeds 5MB, split into parts",
		"task_id", taskID, "parts", partNo)
	return mainPath, nil
}

// splitAndWrite 将超大内容分片写入多个 OSS 文件
func (s *Service) splitAndWrite(ctx context.Context, taskID string, draftVersion int, content []byte) error {
	_, err := s.writeLargeContent(ctx, taskID, draftVersion, content)
	return err
}
