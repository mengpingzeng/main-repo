package a4md

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
)

// MockStorage 用于单元测试的 Storage 实现
type MockStorage struct {
	mu   sync.Mutex
	Data map[string][]byte
}

func NewMockStorage() *MockStorage {
	return &MockStorage{Data: make(map[string][]byte)}
}

func (m *MockStorage) Put(ctx context.Context, path string, data io.Reader) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	content, _ := io.ReadAll(data)
	m.Data[path] = content
	return nil
}

func (m *MockStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	content, ok := m.Data[path]
	if !ok {
		return nil, &mockNotFoundError{path: path}
	}
	return io.NopCloser(bytes.NewReader(content)), nil
}

func (m *MockStorage) Delete(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Data, path)
	return nil
}

func (m *MockStorage) List(ctx context.Context, prefix string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var paths []string
	for p := range m.Data {
		if strings.HasPrefix(p, prefix) {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

func (m *MockStorage) Exists(ctx context.Context, path string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.Data[path]
	return ok, nil
}

type mockNotFoundError struct{ path string }

func (e *mockNotFoundError) Error() string { return "not found: " + e.path }

// FailingStorage 注入失败，测试降级逻辑
// 内嵌 MockStorage，按需在特定路径上注入失败
type FailingStorage struct {
	*MockStorage
	GetFailPaths map[string]bool // 指定路径的 Get 返回失败
	GetFailAll   bool            // 为 true 时所有 Get 返回失败
	PutFailCount int             // 前 N 次 Put 失败
	putCalled    int
	mu           sync.Mutex
}

func NewFailingStorage() *FailingStorage {
	return &FailingStorage{
		MockStorage:  NewMockStorage(),
		GetFailPaths: make(map[string]bool),
	}
}

func (f *FailingStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	if f.GetFailAll || f.GetFailPaths[path] {
		return nil, ErrOSSReadFail
	}
	return f.MockStorage.Get(ctx, path)
}

func (f *FailingStorage) Put(ctx context.Context, path string, data io.Reader) error {
	f.mu.Lock()
	f.putCalled++
	call := f.putCalled
	f.mu.Unlock()
	if call <= f.PutFailCount {
		return ErrOSSWriteFail
	}
	return f.MockStorage.Put(ctx, path, data)
}

// MockStatsRepo 模拟 MySQL platform_stats 查询
type MockStatsRepo struct {
	Stats map[string][]StatItem // key: taskID 或 taskID+"#"+period
}

func NewMockStatsRepo() *MockStatsRepo {
	return &MockStatsRepo{Stats: make(map[string][]StatItem)}
}

func (m *MockStatsRepo) GetStatsByTaskID(ctx context.Context, taskID string) ([]StatItem, error) {
	return m.Stats[taskID], nil
}

func (m *MockStatsRepo) GetStatsByPeriod(ctx context.Context, taskID string, period string) ([]StatItem, error) {
	return m.Stats[taskID+"#"+period], nil
}
