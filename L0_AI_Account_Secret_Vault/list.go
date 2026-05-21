package vault

import (
	"context"
	"fmt"
	"time"
)

const defaultPageLimit = 20
const maxPageLimit = 100

func (v *RealSecretVault) List(ctx context.Context, req ListRequest) (*ListResponse, error) {
	if req.UID == "" {
		creds, err := v.store.FindAll(ctx, req.Platform, req.Offset, req.Limit)
		if err != nil {
			return nil, fmt.Errorf("list all credentials: %w", err)
		}
		accounts := make([]AccountSummary, 0, len(creds))
		for _, cred := range creds {
			accounts = append(accounts, AccountSummary{
				AccountID:     cred.AccountID,
				UID:           cred.UID,
				Platform:      cred.Platform,
				MaskedDisplay: cred.MaskedDisplay,
				BoundAt:       cred.CreatedAt.Format(time.RFC3339),
				UpdatedAt:     cred.UpdatedAt.Format(time.RFC3339),
			})
		}
		return &ListResponse{Accounts: accounts, Total: len(accounts)}, nil
	}

	if req.Limit <= 0 {
		req.Limit = defaultPageLimit
	}
	if req.Limit > maxPageLimit {
		req.Limit = maxPageLimit
	}

	total, err := v.store.CountByUID(ctx, req.UID, req.Platform)
	if err != nil {
		return nil, fmt.Errorf("count credentials: %w", err)
	}

	creds, err := v.store.FindByUID(ctx, req.UID, req.Platform, req.Offset, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}

	accounts := make([]AccountSummary, 0, len(creds))
	for _, cred := range creds {
		accounts = append(accounts, AccountSummary{
			AccountID:     cred.AccountID,
			UID:           cred.UID,
			Platform:      cred.Platform,
			MaskedDisplay: cred.MaskedDisplay,
			BoundAt:       cred.CreatedAt.Format(time.RFC3339),
			UpdatedAt:     cred.UpdatedAt.Format(time.RFC3339),
		})
	}

	return &ListResponse{
		Accounts: accounts,
		Total:    total,
	}, nil
}
