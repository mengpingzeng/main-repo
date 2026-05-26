package vault

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
)

// BackfillPlatformAuthorIDs 为历史绑定记录补写平台作者唯一标识。
func (v *RealSecretVault) BackfillPlatformAuthorIDs(ctx context.Context) error {
	for _, platform := range []string{"fanqie", "zhulang"} {
		rows, err := v.store.ListActiveWithoutPlatformAuthorID(ctx, platform)
		if err != nil {
			return fmt.Errorf("list credentials without platform author id (%s): %w", platform, err)
		}
		if len(rows) == 0 {
			continue
		}
		log.Printf("[a1] backfilling platform_author_id for %d %s accounts", len(rows), platform)
		for _, cred := range rows {
			ciphertext, err := base64.StdEncoding.DecodeString(cred.Credential)
			if err != nil {
				return fmt.Errorf("decode credential for %s: %w", cred.AccountID, err)
			}
			plaintext, err := v.encryptor.Decrypt(ctx, ciphertext, "v1")
			if err != nil {
				return fmt.Errorf("decrypt credential for %s: %w", cred.AccountID, err)
			}

			authorID, err := ResolvePlatformAuthorID(ctx, cred.Platform, string(plaintext))
			if err != nil {
				log.Printf("[a1] skip platform_author_id for %s: %v", cred.AccountID, err)
				continue
			}
			if authorID == "" {
				continue
			}
			if err := v.store.UpdatePlatformAuthorID(ctx, cred.AccountID, authorID); err != nil {
				return fmt.Errorf("update platform_author_id for %s: %w", cred.AccountID, err)
			}
		}
	}
	return nil
}
