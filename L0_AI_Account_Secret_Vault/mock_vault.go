package vault

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// MockSecretVault 纯内存实现的 SecretVault。
type MockSecretVault struct {
	mu        sync.RWMutex
	encryptor Encryptor
	store     map[string]*AccountCredential
	auditLog  []AuditEntry
	bindCount int
}

func NewMockSecretVault() *MockSecretVault {
	return &MockSecretVault{
		encryptor: NewMockEncryptor(),
		store:     make(map[string]*AccountCredential),
		auditLog:  make([]AuditEntry, 0),
	}
}

func NewMockSecretVaultWithEncryptor(enc Encryptor) *MockSecretVault {
	return &MockSecretVault{
		encryptor: enc,
		store:     make(map[string]*AccountCredential),
		auditLog:  make([]AuditEntry, 0),
	}
}

func (m *MockSecretVault) Bind(ctx context.Context, req BindRequest) (*BindResponse, error) {
	if req.UID == "" || req.Platform == "" || req.CredentialsPlaintext == "" {
		return nil, ErrInvalidInput
	}

	accountID := req.AccountID
	isNewBinding := true
	if accountID != "" {
		m.mu.RLock()
		_, exists := m.store[accountID]
		m.mu.RUnlock()
		if exists {
			isNewBinding = false
		}
	} else {
		m.bindCount++
		accountID = fmt.Sprintf("acc_%s_%s_%d", req.UID, req.Platform, m.bindCount)
	}

	encryptResult, err := m.encryptor.Encrypt(ctx, []byte(req.CredentialsPlaintext))
	if err != nil {
		return nil, err
	}

	maskedDisplay := req.MaskedDisplay
	if maskedDisplay == "" {
		maskedDisplay = generateMaskedDisplay(req.Platform, req.CredentialsPlaintext)
	}

	now := time.Now().UTC()
	cred := &AccountCredential{
		AccountID:     accountID,
		UID:           req.UID,
		Platform:      req.Platform,
		Credential:    base64.StdEncoding.EncodeToString(encryptResult.Ciphertext),
		MaskedDisplay: maskedDisplay,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	m.mu.Lock()
	m.store[cred.AccountID] = cred
	m.auditLog = append(m.auditLog, AuditEntry{
		AccountID: accountID,
		Action:    "bind",
		Caller:    req.Caller,
		Result:    "success",
		CreatedAt: now,
	})
	m.mu.Unlock()

	return &BindResponse{
		AccountID:     accountID,
		UID:           req.UID,
		Platform:      req.Platform,
		MaskedDisplay: maskedDisplay,
		IsNewBinding:  isNewBinding,
		BoundAt:       now.Format(time.RFC3339),
	}, nil
}

func (m *MockSecretVault) Unbind(ctx context.Context, req UnbindRequest) (*UnbindResponse, error) {
	if req.AccountID == "" {
		return nil, ErrInvalidInput
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cred := range m.store {
		if cred.AccountID == req.AccountID {
			now := time.Now().UTC()
			cred.Credential = ""
			cred.UpdatedAt = now
			m.auditLog = append(m.auditLog, AuditEntry{
				AccountID: req.AccountID,
				Action:    "unbind",
				Caller:    req.Caller,
				Result:    "success",
				CreatedAt: now,
			})
			return &UnbindResponse{
				AccountID: req.AccountID,
				UnboundAt: now.Format(time.RFC3339),
			}, nil
		}
	}

	return &UnbindResponse{
		AccountID: req.AccountID,
		UnboundAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (m *MockSecretVault) List(ctx context.Context, req ListRequest) (*ListResponse, error) {
	if req.UID == "" {
		return nil, ErrInvalidInput
	}

	const defaultLimit = 20
	const maxLimit = 100
	if req.Limit <= 0 {
		req.Limit = defaultLimit
	}
	if req.Limit > maxLimit {
		req.Limit = maxLimit
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var matched []AccountSummary
	for _, cred := range m.store {
		if cred.UID != req.UID {
			continue
		}
		if req.Platform != "" && cred.Platform != req.Platform {
			continue
		}
		if cred.Credential == "" {
			continue
		}
		matched = append(matched, AccountSummary{
			AccountID:     cred.AccountID,
			UID:           cred.UID,
			Platform:      cred.Platform,
			MaskedDisplay: cred.MaskedDisplay,
			BoundAt:       cred.CreatedAt.Format(time.RFC3339),
			UpdatedAt:     cred.UpdatedAt.Format(time.RFC3339),
		})
	}

	total := len(matched)

	start := req.Offset
	if start > total {
		start = total
	}
	end := start + req.Limit
	if end > total {
		end = total
	}

	paged := matched[start:end]
	if paged == nil {
		paged = []AccountSummary{}
	}

	return &ListResponse{Accounts: paged, Total: total}, nil
}

func (m *MockSecretVault) GetCredentials(ctx context.Context, req GetCredentialsRequest) (*GetCredentialsResponse, error) {
	if req.Caller != "c1_publisher" {
		m.mu.Lock()
		m.auditLog = append(m.auditLog, AuditEntry{
			AccountID: req.AccountID,
			Action:    "get_credentials_denied",
			Caller:    req.Caller,
			Result:    "forbidden",
			CreatedAt: time.Now().UTC(),
		})
		m.mu.Unlock()
		return nil, ErrUnauthorized
	}

	if req.UID == "" {
		return nil, ErrInvalidInput
	}

	m.mu.RLock()

	var cred *AccountCredential
	for _, c := range m.store {
		if c.AccountID == req.AccountID {
			cred = c
			break
		}
	}

	if cred == nil {
		m.mu.RUnlock()
		return nil, ErrAccountNotFound
	}

	if cred.Credential == "" {
		m.mu.RUnlock()
		return nil, ErrAccountNotFound
	}

	if cred.UID != req.UID {
		m.mu.RUnlock()
		m.mu.Lock()
		m.auditLog = append(m.auditLog, AuditEntry{
			AccountID: req.AccountID,
			Action:    "get_credentials_denied",
			Caller:    req.Caller,
			Result:    "forbidden",
			ErrorCode: "UID_MISMATCH",
			CreatedAt: time.Now().UTC(),
		})
		m.mu.Unlock()
		return nil, ErrUnauthorized
	}

	credentialB64 := cred.Credential
	uid := cred.UID
	platform := cred.Platform
	maskedDisplay := cred.MaskedDisplay
	m.mu.RUnlock()

	ciphertext, err := base64.StdEncoding.DecodeString(credentialB64)
	if err != nil {
		m.mu.Lock()
		m.auditLog = append(m.auditLog, AuditEntry{
			AccountID: req.AccountID,
			Action:    "get_credentials",
			Caller:    req.Caller,
			Result:    "error",
			ErrorCode: ErrorCode(ErrDecryptFailed),
			CreatedAt: time.Now().UTC(),
		})
		m.mu.Unlock()
		return nil, ErrDecryptFailed
	}

	plaintext, err := m.encryptor.Decrypt(ctx, ciphertext, mockKeyVersion)
	if err != nil {
		m.mu.Lock()
		m.auditLog = append(m.auditLog, AuditEntry{
			AccountID: req.AccountID,
			Action:    "get_credentials",
			Caller:    req.Caller,
			Result:    "error",
			ErrorCode: ErrorCode(err),
			CreatedAt: time.Now().UTC(),
		})
		m.mu.Unlock()
		return nil, err
	}

	m.mu.Lock()
	m.auditLog = append(m.auditLog, AuditEntry{
		AccountID: req.AccountID,
		Action:    "get_credentials",
		Caller:    req.Caller,
		Result:    "success",
		CreatedAt: time.Now().UTC(),
	})
	m.mu.Unlock()

	return &GetCredentialsResponse{
		AccountID:       req.AccountID,
		UID:             uid,
		Platform:        platform,
		Credentials:     string(plaintext),
		MaskedDisplay:   maskedDisplay,
		SecurityWarning: "SENSITIVE: DO NOT LOG",
	}, nil
}

func (m *MockSecretVault) GetCredentialsBatch(ctx context.Context, req GetCredentialsBatchRequest) (*GetCredentialsBatchResponse, error) {
	if req.Caller != "c1_publisher" {
		return nil, ErrUnauthorized
	}

	var results []CredentialsResult
	for _, accountID := range req.AccountIDs {
		credResp, err := m.GetCredentials(ctx, GetCredentialsRequest{
			AccountID: accountID,
			UID:       req.UID,
			Caller:    req.Caller,
		})
		if err != nil {
			results = append(results, CredentialsResult{
				AccountID: accountID,
				Error:     ErrorCode(err),
			})
			continue
		}
		results = append(results, CredentialsResult{
			AccountID:       credResp.AccountID,
			UID:             credResp.UID,
			Platform:        credResp.Platform,
			Credentials:     credResp.Credentials,
			SecurityWarning: credResp.SecurityWarning,
		})
	}

	return &GetCredentialsBatchResponse{Results: results}, nil
}

func (m *MockSecretVault) Health(ctx context.Context) error {
	return nil
}

// CheckCookieHealth mock 实现：直接对凭证做字符串解析，无需解密。
func (m *MockSecretVault) CheckCookieHealth(ctx context.Context, req CheckCookieHealthRequest) (*CheckCookieHealthResponse, error) {
	m.mu.RLock()
	var cred *AccountCredential
	for _, c := range m.store {
		if c.AccountID == req.AccountID {
			cred = c
			break
		}
	}
	m.mu.RUnlock()

	if cred == nil || cred.Credential == "" {
		return nil, ErrAccountNotFound
	}

	if req.UID != "" && cred.UID != req.UID {
		return nil, ErrUnauthorized
	}

	// Mock 模式下凭证以明文（base64 of plaintext）存储
	ciphertext, err := base64.StdEncoding.DecodeString(cred.Credential)
	if err != nil {
		return nil, ErrDecryptFailed
	}
	plaintext, err := m.encryptor.Decrypt(ctx, ciphertext, mockKeyVersion)
	if err != nil {
		return nil, err
	}

	valid, err := checkPlatformCookieExpiry(ctx, cred.Platform, string(plaintext))
	if err != nil {
		return nil, err
	}

	return &CheckCookieHealthResponse{
		AccountID: req.AccountID,
		Valid:     valid,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// GetCredentialForOwner mock 实现。
func (m *MockSecretVault) GetCredentialForOwner(ctx context.Context, accountID, uid string) (*GetCredentialsResponse, error) {
	return m.GetCredentials(ctx, GetCredentialsRequest{
		AccountID: accountID,
		UID:       uid,
		Caller:    "c1_publisher",
	})
}

func (m *MockSecretVault) Close() error {
	return nil
}

func (m *MockSecretVault) GetAuditLog() []AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]AuditEntry, len(m.auditLog))
	copy(result, m.auditLog)
	return result
}

func (m *MockSecretVault) GetStore() map[string]*AccountCredential {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*AccountCredential, len(m.store))
	for k, v := range m.store {
		cred := *v
		result[k] = &cred
	}
	return result
}

func (m *MockSecretVault) SetStoreEntry(key string, cred *AccountCredential) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[key] = cred
}
