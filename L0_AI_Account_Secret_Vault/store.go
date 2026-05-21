package vault

import (
	"context"
	"database/sql"
	"fmt"
)

// CredentialStore 凭证存储的数据库操作层。
type CredentialStore struct {
	db *sql.DB
}

func NewCredentialStore(db *sql.DB) *CredentialStore {
	return &CredentialStore{db: db}
}

// Upsert 插入或更新凭证（利用唯一键实现幂等）。
func (s *CredentialStore) Upsert(ctx context.Context, cred *AccountCredential) error {
	query := `
		INSERT INTO a1_credentials
			(account_id, uid, platform, credential, masked_display, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			credential = VALUES(credential),
			masked_display = VALUES(masked_display),
			updated_at = VALUES(updated_at)
	`
	_, err := s.db.ExecContext(ctx, query,
		cred.AccountID, cred.UID, cred.Platform,
		cred.Credential, cred.MaskedDisplay, cred.CreatedAt, cred.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert a1_credential: %w", err)
	}
	return nil
}

// FindByUID 查询用户的所有已绑定账号（不含 credential 列，支持分页）。
func (s *CredentialStore) FindByUID(ctx context.Context, uid string, platform string, offset int, limit int) ([]*AccountCredential, error) {
	query := `
		SELECT account_id, uid, platform, masked_display, created_at, updated_at
		FROM a1_credentials
		WHERE uid = ?
		  AND credential IS NOT NULL AND credential != ''
	`
	args := []interface{}{uid}
	if platform != "" {
		query += " AND platform = ?"
		args = append(args, platform)
	}
	query += " ORDER BY updated_at DESC"
	if limit > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query credentials by uid: %w", err)
	}
	defer rows.Close()

	var result []*AccountCredential
	for rows.Next() {
		cred := &AccountCredential{}
		if err := rows.Scan(
			&cred.AccountID, &cred.UID, &cred.Platform,
			&cred.MaskedDisplay, &cred.CreatedAt, &cred.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}
		result = append(result, cred)
	}
	return result, rows.Err()
}

// CountByUID 查询用户已绑定账号总数（用于分页 Total 字段）。
func (s *CredentialStore) CountByUID(ctx context.Context, uid string, platform string) (int, error) {
	query := `SELECT COUNT(*) FROM a1_credentials WHERE uid = ? AND credential IS NOT NULL AND credential != ''`
	args := []interface{}{uid}
	if platform != "" {
		query += " AND platform = ?"
		args = append(args, platform)
	}
	var total int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&total)
	return total, err
}

// FindByAccountID 按 account_id 查询完整凭证（包含 credential，仅 GetCredentials 用）。
func (s *CredentialStore) FindByAccountID(ctx context.Context, accountID string) (*AccountCredential, error) {
	query := `
		SELECT account_id, uid, platform, credential, masked_display, created_at, updated_at
		FROM a1_credentials
		WHERE account_id = ?
	`
	cred := &AccountCredential{}
	err := s.db.QueryRowContext(ctx, query, accountID).Scan(
		&cred.AccountID, &cred.UID, &cred.Platform,
		&cred.Credential, &cred.MaskedDisplay, &cred.CreatedAt, &cred.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: %s", ErrAccountNotFound, accountID)
	}
	if err != nil {
		return nil, fmt.Errorf("query credential by id: %w", err)
	}
	return cred, nil
}

// FindByUIDWithCiphertext 查询用户指定平台的完整凭证（含 credential 列，用于 bind 时检查 IsNewBinding）。
func (s *CredentialStore) FindByUIDWithCiphertext(ctx context.Context, uid string, platform string) (*AccountCredential, error) {
	query := `
		SELECT account_id, uid, platform, credential, masked_display, created_at, updated_at
		FROM a1_credentials
		WHERE uid = ? AND platform = ?
	`
	cred := &AccountCredential{}
	err := s.db.QueryRowContext(ctx, query, uid, platform).Scan(
		&cred.AccountID, &cred.UID, &cred.Platform,
		&cred.Credential, &cred.MaskedDisplay, &cred.CreatedAt, &cred.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query credential by uid+platform: %w", err)
	}
	return cred, nil
}

// SoftDelete 软删除凭证（清空 credential）。
func (s *CredentialStore) SoftDelete(ctx context.Context, accountID string) error {
	query := `
		UPDATE a1_credentials
		SET credential = ''
		WHERE account_id = ?
	`
	_, err := s.db.ExecContext(ctx, query, accountID)
	return err
}

// FindAll 管理员查询所有已绑定账号（分页）。
func (s *CredentialStore) FindAll(ctx context.Context, platform string, offset int, limit int) ([]*AccountCredential, error) {
	query := `
		SELECT account_id, uid, platform, masked_display, created_at, updated_at
		FROM a1_credentials
		WHERE credential IS NOT NULL AND credential != ''
	`
	args := []interface{}{}
	if platform != "" {
		query += " AND platform = ?"
		args = append(args, platform)
	}
	query += " ORDER BY updated_at DESC"
	if limit > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query all credentials: %w", err)
	}
	defer rows.Close()

	var result []*AccountCredential
	for rows.Next() {
		cred := &AccountCredential{}
		if err := rows.Scan(
			&cred.AccountID, &cred.UID, &cred.Platform,
			&cred.MaskedDisplay, &cred.CreatedAt, &cred.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}
		result = append(result, cred)
	}
	return result, rows.Err()
}
