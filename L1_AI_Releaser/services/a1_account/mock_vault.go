// Package a1_account 提供 MockSecretVault 实现。
package a1_account

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockSecretVault 纯内存实现的 SecretVault。
// 用于 C1 开发和测试阶段，不依赖 MySQL/KMS。
type MockSecretVault struct {
	mu               sync.RWMutex
	store            map[string]*mockCredRecord // key: uid|platform
	failAllMode      bool
	failAllErr       error
	perAccountFail   map[string]error // key: accountID
}

type mockCredRecord struct {
	AccountID     string
	UID           string
	Platform      string
	MaskedDisplay string
	Credentials   string // mock 模式明文存储
	BoundAt       time.Time
}

func NewMockSecretVault() *MockSecretVault {
	return &MockSecretVault{
		store:          make(map[string]*mockCredRecord),
		perAccountFail: make(map[string]error),
	}
}

func (m *MockSecretVault) Bind(_ context.Context, req BindRequest) (*BindResponse, error) {
	if req.UID == "" || req.Platform == "" || req.CredentialsPlaintext == "" {
		return nil, ErrInvalidInput
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := req.UID + "|" + req.Platform
	_, exists := m.store[key]
	accountID := fmt.Sprintf("acc_%s_%s", req.UID, req.Platform)
	masked := req.MaskedDisplay
	if masked == "" {
		masked = req.Platform + "_bound"
	}

	now := time.Now().UTC()
	m.store[key] = &mockCredRecord{
		AccountID:     accountID,
		UID:           req.UID,
		Platform:      req.Platform,
		MaskedDisplay: masked,
		Credentials:   req.CredentialsPlaintext,
		BoundAt:       now,
	}

	return &BindResponse{
		AccountID:     accountID,
		UID:           req.UID,
		Platform:      req.Platform,
		MaskedDisplay: masked,
		IsNewBinding:  !exists,
		BoundAt:       now.Format(time.RFC3339),
	}, nil
}

func (m *MockSecretVault) GetCredentials(_ context.Context, req GetCredentialsRequest) (*GetCredentialsResponse, error) {
	if req.Caller != "c1_publisher" {
		return nil, ErrUnauthorized
	}
	if req.UID == "" {
		return nil, ErrInvalidInput
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.failAllMode {
		return nil, m.failAllErr
	}

	if err, ok := m.perAccountFail[req.AccountID]; ok {
		return nil, err
	}

	for _, rec := range m.store {
		if rec.AccountID == req.AccountID {
			if rec.UID != req.UID {
				return nil, ErrUnauthorized
			}
			return &GetCredentialsResponse{
				AccountID:       rec.AccountID,
				UID:             rec.UID,
				Platform:        rec.Platform,
				Credentials:     rec.Credentials,
				SecurityWarning: "SENSITIVE: DO NOT LOG",
				MaskedDisplay:   rec.MaskedDisplay,
			}, nil
		}
	}

	return nil, ErrAccountNotFound
}

func (m *MockSecretVault) GetCredentialsBatch(_ context.Context, req GetCredentialsBatchRequest) (*GetCredentialsBatchResponse, error) {
	var results []CredentialsResult
	for _, aid := range req.AccountIDs {
		cr, err := m.GetCredentials(nil, GetCredentialsRequest{
			AccountID: aid,
			UID:       req.UID,
			Caller:    req.Caller,
		})
		if err != nil {
			code := "UNKNOWN"
			switch {
			case err == ErrAccountNotFound:
				code = "ACCOUNT_NOT_FOUND"
			case err == ErrUnauthorized:
				code = "UNAUTHORIZED"
			}
			results = append(results, CredentialsResult{
				AccountID: aid,
				Error:     code,
			})
			continue
		}
		results = append(results, CredentialsResult{
			AccountID:       cr.AccountID,
			UID:             cr.UID,
			Platform:        cr.Platform,
			Credentials:     cr.Credentials,
			SecurityWarning: cr.SecurityWarning,
			MaskedDisplay:   cr.MaskedDisplay,
		})
	}
	return &GetCredentialsBatchResponse{Results: results}, nil
}

func (m *MockSecretVault) Health(_ context.Context) error { return nil }
func (m *MockSecretVault) Close() error                    { return nil }

// --- 测试辅助 ---

func (m *MockSecretVault) SetFailAll(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failAllMode = true
	m.failAllErr = err
}

func (m *MockSecretVault) ClearFailAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failAllMode = false
	m.failAllErr = nil
}

func (m *MockSecretVault) SetFailAccount(accountID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.perAccountFail[accountID] = err
}

func (m *MockSecretVault) ClearFailForAccount(accountID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.perAccountFail, accountID)
}
