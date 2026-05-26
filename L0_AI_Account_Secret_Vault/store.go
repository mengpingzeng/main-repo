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
			(account_id, uid, platform, credential, credential_fingerprint, platform_author_id, masked_display, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			credential = VALUES(credential),
			credential_fingerprint = VALUES(credential_fingerprint),
			platform_author_id = VALUES(platform_author_id),
			masked_display = VALUES(masked_display),
			updated_at = VALUES(updated_at)
	`
	_, err := s.db.ExecContext(ctx, query,
		cred.AccountID, cred.UID, cred.Platform,
		cred.Credential, cred.CredentialFingerprint, cred.PlatformAuthorID, cred.MaskedDisplay, cred.CreatedAt, cred.UpdatedAt,
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

// FindByDisplayName 查询同平台下是否已有相同账号名（全局范围，跨用户）。
// 用于绑定前的唯一性校验。excludeAccountID 不为空时排除自身（用于更新场景）。
func (s *CredentialStore) FindByDisplayName(ctx context.Context, platform, maskedDisplay, excludeAccountID string) (*AccountCredential, error) {
	query := `
		SELECT account_id, uid, platform, masked_display, created_at, updated_at
		FROM a1_credentials
		WHERE platform = ?
		  AND masked_display = ?
		  AND credential IS NOT NULL AND credential != ''
	`
	args := []interface{}{platform, maskedDisplay}
	if excludeAccountID != "" {
		query += " AND account_id != ?"
		args = append(args, excludeAccountID)
	}
	query += " LIMIT 1"

	cred := &AccountCredential{}
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&cred.AccountID, &cred.UID, &cred.Platform,
		&cred.MaskedDisplay, &cred.CreatedAt, &cred.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query credential by display name: %w", err)
	}
	return cred, nil
}

// FindByFingerprint 查询同平台下是否已有相同凭证指纹（全局范围，跨用户）。
// excludeAccountID 不为空时排除自身（用于更新场景）。
func (s *CredentialStore) FindByFingerprint(ctx context.Context, platform, fingerprint, excludeAccountID string) (*AccountCredential, error) {
	if fingerprint == "" {
		return nil, nil
	}

	query := `
		SELECT account_id, uid, platform, masked_display, created_at, updated_at
		FROM a1_credentials
		WHERE platform = ?
		  AND credential_fingerprint = ?
		  AND credential IS NOT NULL AND credential != ''
	`
	args := []interface{}{platform, fingerprint}
	if excludeAccountID != "" {
		query += " AND account_id != ?"
		args = append(args, excludeAccountID)
	}
	query += " LIMIT 1"

	cred := &AccountCredential{}
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&cred.AccountID, &cred.UID, &cred.Platform,
		&cred.MaskedDisplay, &cred.CreatedAt, &cred.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query credential by fingerprint: %w", err)
	}
	return cred, nil
}

// FindByUIDPlatformAuthorID 查找同用户同平台下已记录的平台作者 ID（含已解绑记录）。
func (s *CredentialStore) FindByUIDPlatformAuthorID(ctx context.Context, uid, platform, platformAuthorID string) (*AccountCredential, error) {
	if platformAuthorID == "" {
		return nil, nil
	}
	query := `
		SELECT account_id, uid, platform, credential, platform_author_id, masked_display, created_at, updated_at
		FROM a1_credentials
		WHERE uid = ? AND platform = ? AND platform_author_id = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`
	cred := &AccountCredential{}
	var credential sql.NullString
	err := s.db.QueryRowContext(ctx, query, uid, platform, platformAuthorID).Scan(
		&cred.AccountID, &cred.UID, &cred.Platform,
		&credential, &cred.PlatformAuthorID, &cred.MaskedDisplay, &cred.CreatedAt, &cred.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query by platform author id: %w", err)
	}
	if credential.Valid {
		cred.Credential = credential.String
	}
	return cred, nil
}

// FindActiveByPlatformAuthorID 查找同平台下仍有效的相同平台作者 ID（跨用户去重）。
func (s *CredentialStore) FindActiveByPlatformAuthorID(ctx context.Context, platform, platformAuthorID, excludeAccountID string) (*AccountCredential, error) {
	if platformAuthorID == "" {
		return nil, nil
	}
	query := `
		SELECT account_id, uid, platform, masked_display, created_at, updated_at
		FROM a1_credentials
		WHERE platform = ?
		  AND platform_author_id = ?
		  AND credential IS NOT NULL AND credential != ''
	`
	args := []interface{}{platform, platformAuthorID}
	if excludeAccountID != "" {
		query += " AND account_id != ?"
		args = append(args, excludeAccountID)
	}
	query += " LIMIT 1"

	cred := &AccountCredential{}
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&cred.AccountID, &cred.UID, &cred.Platform,
		&cred.MaskedDisplay, &cred.CreatedAt, &cred.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query active platform author id: %w", err)
	}
	cred.PlatformAuthorID = platformAuthorID
	return cred, nil
}

// UpdatePlatformAuthorID 写入平台作者唯一标识。
func (s *CredentialStore) UpdatePlatformAuthorID(ctx context.Context, accountID, platformAuthorID string) error {
	query := `
		UPDATE a1_credentials
		SET platform_author_id = ?, updated_at = UTC_TIMESTAMP()
		WHERE account_id = ?
	`
	_, err := s.db.ExecContext(ctx, query, platformAuthorID, accountID)
	return err
}

// ListActiveWithoutPlatformAuthorID 列出仍有效但未写入平台作者 ID 的记录（启动回填用）。
func (s *CredentialStore) ListActiveWithoutPlatformAuthorID(ctx context.Context, platform string) ([]*AccountCredential, error) {
	query := `
		SELECT account_id, uid, platform, credential, masked_display, created_at, updated_at
		FROM a1_credentials
		WHERE credential IS NOT NULL AND credential != ''
		  AND (platform_author_id IS NULL OR platform_author_id = '')
	`
	args := []interface{}{}
	if platform != "" {
		query += " AND platform = ?"
		args = append(args, platform)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query credentials without platform author id: %w", err)
	}
	defer rows.Close()

	var result []*AccountCredential
	for rows.Next() {
		cred := &AccountCredential{}
		if err := rows.Scan(
			&cred.AccountID, &cred.UID, &cred.Platform,
			&cred.Credential, &cred.MaskedDisplay, &cred.CreatedAt, &cred.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}
		result = append(result, cred)
	}
	return result, rows.Err()
}

// ListActiveWithoutFingerprint 列出仍有效但未写入指纹的历史记录（用于启动回填）。
func (s *CredentialStore) ListActiveWithoutFingerprint(ctx context.Context) ([]*AccountCredential, error) {
	query := `
		SELECT account_id, uid, platform, credential, masked_display, created_at, updated_at
		FROM a1_credentials
		WHERE credential IS NOT NULL AND credential != ''
		  AND (credential_fingerprint IS NULL OR credential_fingerprint = '')
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query credentials without fingerprint: %w", err)
	}
	defer rows.Close()

	var result []*AccountCredential
	for rows.Next() {
		cred := &AccountCredential{}
		if err := rows.Scan(
			&cred.AccountID, &cred.UID, &cred.Platform,
			&cred.Credential, &cred.MaskedDisplay, &cred.CreatedAt, &cred.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}
		result = append(result, cred)
	}
	return result, rows.Err()
}

// UpdateFingerprint 为已有记录写入凭证指纹。
func (s *CredentialStore) UpdateFingerprint(ctx context.Context, accountID, fingerprint string) error {
	query := `
		UPDATE a1_credentials
		SET credential_fingerprint = ?, updated_at = UTC_TIMESTAMP()
		WHERE account_id = ?
	`
	_, err := s.db.ExecContext(ctx, query, fingerprint, accountID)
	return err
}

// SoftDelete 软删除凭证（清空 credential）。
func (s *CredentialStore) SoftDelete(ctx context.Context, accountID string) error {
	query := `
		UPDATE a1_credentials
		SET credential = '', credential_fingerprint = NULL
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
