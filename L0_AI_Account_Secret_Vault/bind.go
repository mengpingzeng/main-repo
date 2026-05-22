package vault

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"
)

func (v *RealSecretVault) Bind(ctx context.Context, req BindRequest) (*BindResponse, error) {
	if req.UID == "" || req.Platform == "" || req.CredentialsPlaintext == "" {
		return nil, ErrInvalidInput
	}

	isNewBinding := true
	accountID := req.AccountID
	if accountID != "" {
		existing, err := v.store.FindByAccountID(ctx, accountID)
		if err != nil {
			return nil, fmt.Errorf("find account: %w", err)
		}
		if existing != nil {
			isNewBinding = false
		}
	} else {
		accountID = generateAccountID()
	}

	maskedDisplay := req.MaskedDisplay
	if maskedDisplay == "" {
		maskedDisplay = generateMaskedDisplay(req.Platform, req.CredentialsPlaintext)
	}

	// 同平台账号名全局唯一校验（更新自身时排除自己）
	conflict, err := v.store.FindByDisplayName(ctx, req.Platform, maskedDisplay, accountID)
	if err != nil {
		return nil, fmt.Errorf("check display name uniqueness: %w", err)
	}
	if conflict != nil {
		return nil, ErrDuplicateDisplayName
	}

	encryptResult, err := v.encryptor.Encrypt(ctx, []byte(req.CredentialsPlaintext))
	if err != nil {
		v.audit.Record(ctx, AuditEntry{
			AccountID: accountID,
			Action:    "bind",
			Caller:    req.Caller,
			Result:    "error",
			ErrorCode: ErrorCode(err),
		})
		return nil, err
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

	if err := v.store.Upsert(ctx, cred); err != nil {
		v.audit.Record(ctx, AuditEntry{
			AccountID: accountID,
			Action:    "bind",
			Caller:    req.Caller,
			Result:    "error",
			ErrorCode: ErrorCode(err),
		})
		return nil, fmt.Errorf("store credential: %w", err)
	}

	v.audit.Record(ctx, AuditEntry{
		AccountID: accountID,
		Action:    "bind",
		Caller:    req.Caller,
		Result:    "success",
	})

	return &BindResponse{
		AccountID:     accountID,
		UID:           req.UID,
		Platform:      req.Platform,
		MaskedDisplay: maskedDisplay,
		IsNewBinding:  isNewBinding,
		BoundAt:       now.Format(time.RFC3339),
	}, nil
}
