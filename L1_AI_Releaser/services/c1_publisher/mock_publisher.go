// Package c1_publisher 提供 MockPublisher 实现（供 Workflow Engine 开发使用）。
package c1_publisher

import (
	"context"
	"fmt"
)

// MockPublisher 纯内存实现的 Publisher。
// 不依赖 A1、平台 API、MySQL。用于 Workflow Engine 团队开发。
type MockPublisher struct {
	failAll       bool
	failOneOf     map[string]bool
	callCount     int
	publishResult []PublishResult
}

func NewMockPublisher() *MockPublisher {
	return &MockPublisher{}
}

func (m *MockPublisher) Publish(_ context.Context, req PublishRequest) (*PublishResponse, error) {
	m.callCount++

	var results []PublishResult
	mockPostID := 1

	for _, acc := range req.Accounts {
		platform := acc.Platform
		status := "ok"
		errorCode := ""
		errMsg := ""
		postID := fmt.Sprintf("mock_%s_%d", platform, mockPostID)
		mockPostID++

		if m.failAll {
			status = "fail"
			errorCode = "MOCK_FAIL"
			errMsg = "mock all fail"
			postID = ""
		} else if m.failOneOf[acc.AccountID] {
			status = "fail"
			errorCode = "MOCK_ACCOUNT_FAIL"
			errMsg = fmt.Sprintf("mock fail for account %s", acc.AccountID)
			postID = ""
		}

		results = append(results, PublishResult{
			AccountID:    acc.AccountID,
			UID:          acc.UID,
			Platform:     platform,
			Status:       status,
			PostID:       postID,
			ErrorCode:    errorCode,
			ErrorMessage: errMsg,
		})
	}

	succeeded, failed := 0, 0
	for _, r := range results {
		if r.Status == "ok" {
			succeeded++
		} else {
			failed++
		}
	}

	m.publishResult = results
	return &PublishResponse{
		TaskID:  req.TaskID,
		Results: results,
		Summary: PublishSummary{
			Total:     len(results),
			Succeeded: succeeded,
			Failed:    failed,
		},
	}, nil
}

func (m *MockPublisher) Health(_ context.Context) error { return nil }
func (m *MockPublisher) Close() error                    { return nil }

func (m *MockPublisher) SetFailAll()                           { m.failAll = true }
func (m *MockPublisher) SetFailAccount(accountID string) {
	if m.failOneOf == nil {
		m.failOneOf = make(map[string]bool)
	}
	m.failOneOf[accountID] = true
}
func (m *MockPublisher) GetCallCount() int                   { return m.callCount }
func (m *MockPublisher) GetLastResults() []PublishResult     { return m.publishResult }
