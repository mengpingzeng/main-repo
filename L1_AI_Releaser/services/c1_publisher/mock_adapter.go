// Package c1_publisher 提供 MockPublishAdapter 和 MockStatsAdapter 实现。
package c1_publisher

import (
	"context"
	"fmt"
	"sync"
)

// MockPublishAdapter 纯内存实现的 PublishAdapter。
//
// 使用方式：
//  1. 正常模式：返回固定 post_id，模拟成功
//  2. 全局错误注入：SetFailMode 让所有调用返回相同错误
//  3. 按账号错误注入：SetFailForAccount 对不同账号注入不同错误
type MockPublishAdapter struct {
	platform string

	mu             sync.RWMutex
	failMode       bool
	failCode       string
	failMsg        string
	callCount      int
	calls          []mockCallRecord
	perAccountFail map[string]mockFailConfig
}

type mockCallRecord struct {
	Product ProductContent
}

type mockFailConfig struct {
	Code string
	Msg  string
}

func NewMockPublishAdapter(platform string) *MockPublishAdapter {
	return &MockPublishAdapter{
		platform:       platform,
		calls:          make([]mockCallRecord, 0),
		perAccountFail: make(map[string]mockFailConfig),
	}
}

func (m *MockPublishAdapter) Platform() string {
	return m.platform
}

func (m *MockPublishAdapter) CheckInput(_ ProductContent) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.failMode && m.failCode == "INPUT_INVALID" {
		return m.failMsg
	}
	return ""
}

func (m *MockPublishAdapter) Publish(_ context.Context, product ProductContent, credentials string, maskedDisplay string) *PublishResult {
	m.mu.Lock()
	m.callCount++
	m.calls = append(m.calls, mockCallRecord{Product: product})
	failMode := m.failMode
	failCode := m.failCode
	failMsg := m.failMsg
	perAF := m.perAccountFail
	m.mu.Unlock()

	if cfg, ok := perAF[credentials]; ok {
		return &PublishResult{
			Platform:     m.platform,
			Status:       "fail",
			ErrorCode:    cfg.Code,
			ErrorMessage: cfg.Msg,
			MaskedDisplay: maskedDisplay,
		}
	}

	if failMode {
		return &PublishResult{
			Platform:     m.platform,
			Status:       "fail",
			ErrorCode:    failCode,
			ErrorMessage: failMsg,
			MaskedDisplay: maskedDisplay,
		}
	}

	return &PublishResult{
		Platform: m.platform,
		Status:   "ok",
		PostID:   fmt.Sprintf("mock_post_%s_%d", m.platform, m.callCount),
		MaskedDisplay: maskedDisplay,
	}
}

// --- 错误注入 ---

func (m *MockPublishAdapter) SetFailMode(code, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failMode = true
	m.failCode = code
	m.failMsg = msg
}

func (m *MockPublishAdapter) SetFailForAccount(accountID, code, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.perAccountFail == nil {
		m.perAccountFail = make(map[string]mockFailConfig)
	}
	m.perAccountFail[accountID] = mockFailConfig{Code: code, Msg: msg}
}

func (m *MockPublishAdapter) ClearFailForAccount(accountID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.perAccountFail, accountID)
}

func (m *MockPublishAdapter) SetNormalMode() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failMode = false
	m.failCode = ""
	m.failMsg = ""
	m.perAccountFail = make(map[string]mockFailConfig)
}

// --- 调用验证 ---

func (m *MockPublishAdapter) GetCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount
}

func (m *MockPublishAdapter) GetCallProducts() []ProductContent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ProductContent, len(m.calls))
	for i, c := range m.calls {
		result[i] = c.Product
	}
	return result
}
