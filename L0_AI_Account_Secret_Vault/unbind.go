package vault

import (
	"context"
	"time"
)

func (v *RealSecretVault) Unbind(ctx context.Context, req UnbindRequest) (*UnbindResponse, error) {
	if req.AccountID == "" {
		return nil, ErrInvalidInput
	}

	cred, err := v.store.FindByAccountID(ctx, req.AccountID)
	if err != nil {
		return &UnbindResponse{
			AccountID: req.AccountID,
			UnboundAt: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	if req.UID != "" && cred.UID != req.UID {
		return nil, ErrUnauthorized
	}

	if err := v.store.SoftDelete(ctx, cred.AccountID); err != nil {
		return nil, err
	}

	v.audit.Record(ctx, AuditEntry{
		AccountID: req.AccountID,
		Action:    "unbind",
		Caller:    req.Caller,
		Result:    "success",
	})

	return &UnbindResponse{
		AccountID: req.AccountID,
		UnboundAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}
