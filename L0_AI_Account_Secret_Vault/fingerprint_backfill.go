package vault

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
)

// BackfillCredentialFingerprints 为历史绑定记录补写凭证指纹，便于全局去重生效。
func (v *RealSecretVault) BackfillCredentialFingerprints(ctx context.Context) error {
	rows, err := v.store.ListActiveWithoutFingerprint(ctx)
	if err != nil {
		return fmt.Errorf("list credentials without fingerprint: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}

	log.Printf("[a1] backfilling credential fingerprints for %d accounts", len(rows))
	for _, cred := range rows {
		ciphertext, err := base64.StdEncoding.DecodeString(cred.Credential)
		if err != nil {
			return fmt.Errorf("decode credential for %s: %w", cred.AccountID, err)
		}
		plaintext, err := v.encryptor.Decrypt(ctx, ciphertext, "v1")
		if err != nil {
			return fmt.Errorf("decrypt credential for %s: %w", cred.AccountID, err)
		}

		fingerprint := computeCredentialFingerprint(cred.Platform, string(plaintext))
		if err := v.store.UpdateFingerprint(ctx, cred.AccountID, fingerprint); err != nil {
			return fmt.Errorf("update fingerprint for %s: %w", cred.AccountID, err)
		}
	}
	return nil
}
