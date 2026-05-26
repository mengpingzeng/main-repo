package vault

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"
)

func requiresPlatformAuthorID(platform string) bool {
	switch platform {
	case "fanqie", "zhulang":
		return true
	default:
		return false
	}
}

func (v *RealSecretVault) Bind(ctx context.Context, req BindRequest) (*BindResponse, error) {
	if req.UID == "" || req.Platform == "" || req.CredentialsPlaintext == "" {
		return nil, ErrInvalidInput
	}

	platformAuthorID, err := ResolvePlatformAuthorID(ctx, req.Platform, req.CredentialsPlaintext)
	if err != nil {
		return nil, WrapError(ErrPlatformAuthorUnresolved, err)
	}
	if requiresPlatformAuthorID(req.Platform) && platformAuthorID == "" {
		return nil, ErrPlatformAuthorUnresolved
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
	} else if platformAuthorID != "" {
		existing, err := v.store.FindByUIDPlatformAuthorID(ctx, req.UID, req.Platform, platformAuthorID)
		if err != nil {
			return nil, fmt.Errorf("find platform author record: %w", err)
		}
		if existing != nil {
			accountID = existing.AccountID
			isNewBinding = false
		}
	}
	if accountID == "" {
		accountID = generateAccountID()
	}

	maskedDisplay := req.MaskedDisplay
	if maskedDisplay == "" {
		maskedDisplay = generateMaskedDisplay(req.Platform, req.CredentialsPlaintext)
	}

	if platformAuthorID != "" {
		authorConflict, err := v.store.FindActiveByPlatformAuthorID(ctx, req.Platform, platformAuthorID, accountID)
		if err != nil {
			return nil, fmt.Errorf("check platform author uniqueness: %w", err)
		}
		if authorConflict != nil && authorConflict.UID != req.UID {
			return nil, ErrDuplicatePlatformAuthor
		}
	}

	conflict, err := v.store.FindByDisplayName(ctx, req.Platform, maskedDisplay, accountID)
	if err != nil {
		return nil, fmt.Errorf("check display name uniqueness: %w", err)
	}
	if conflict != nil {
		return nil, ErrDuplicateDisplayName
	}

	fingerprint := computeCredentialFingerprint(req.Platform, req.CredentialsPlaintext)

	credConflict, err := v.store.FindByFingerprint(ctx, req.Platform, fingerprint, accountID)
	if err != nil {
		return nil, fmt.Errorf("check credential uniqueness: %w", err)
	}
	if credConflict != nil {
		return nil, ErrDuplicateCredential
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
		AccountID:             accountID,
		UID:                   req.UID,
		Platform:              req.Platform,
		Credential:            base64.StdEncoding.EncodeToString(encryptResult.Ciphertext),
		CredentialFingerprint: fingerprint,
		PlatformAuthorID:      platformAuthorID,
		MaskedDisplay:         maskedDisplay,
		CreatedAt:             now,
		UpdatedAt:             now,
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
