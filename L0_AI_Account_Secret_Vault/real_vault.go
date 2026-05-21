package vault

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"
)

// RealSecretVault 是 SecretVault 的真实实现，组装所有组件。
type RealSecretVault struct {
	cfg          *Config
	encryptor    Encryptor
	store        *CredentialStore
	userStore    *UserStore
	audit        *AuditWriter
	adminAudit   *AdminAuditWriter
	c1CallerID   string
}

func NewRealSecretVault(cfg *Config) (*RealSecretVault, error) {
	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)

	enc, err := NewEncryptor(cfg)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &RealSecretVault{
		cfg:          cfg,
		encryptor:    enc,
		store:        NewCredentialStore(db),
		userStore:    NewUserStore(db),
		audit:        NewAuditWriter(db, cfg.AuditEnabled),
		adminAudit:   NewAdminAuditWriter(db, cfg.AuditEnabled),
		c1CallerID:   cfg.C1CallerIdentifier,
	}, nil
}

func (v *RealSecretVault) Health(ctx context.Context) error {
	return v.encryptor.Health(ctx)
}

// CheckCookieHealth 通过解析存储凭证中的平台 session cookie 来判断登录态是否有效。
// 不发出任何外部 HTTP 请求，纯本地解析。
func (v *RealSecretVault) CheckCookieHealth(ctx context.Context, req CheckCookieHealthRequest) (*CheckCookieHealthResponse, error) {
	// 1. 从数据库读取完整凭证记录（含密文）
	cred, err := v.store.FindByAccountID(ctx, req.AccountID)
	if err != nil {
		return nil, err
	}

	// 2. UID 归属校验：只允许检测自己的账号
	if req.UID != "" && cred.UID != req.UID {
		v.audit.Record(ctx, AuditEntry{
			AccountID: req.AccountID,
			Action:    "check_cookie_health_denied",
			Caller:    "bff",
			Result:    "forbidden",
			ErrorCode: "UID_MISMATCH",
		})
		return nil, ErrUnauthorized
	}

	// 3. 解密凭证
	if cred.Credential == "" {
		return nil, ErrAccountNotFound
	}

	ciphertext, err := base64.StdEncoding.DecodeString(cred.Credential)
	if err != nil {
		return nil, ErrDecryptFailed
	}

	plaintext, err := v.encryptor.Decrypt(ctx, ciphertext, "v1")
	if err != nil {
		return nil, err
	}

	// 4. 向平台发轻量 HTTP 探测，判断 session 是否仍被服务端认可
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
func (v *RealSecretVault) Register(ctx context.Context, username, password, role string) (*User, error) {
	return v.userStore.Create(ctx, username, password, role)
}

func (v *RealSecretVault) Login(ctx context.Context, username, password string) (*User, error) {
	return v.userStore.Authenticate(ctx, username, password)
}

func (v *RealSecretVault) ListUsers(ctx context.Context) ([]AdminUserInfo, error) {
	return v.userStore.ListUsers(ctx)
}

func (v *RealSecretVault) UpdateUser(ctx context.Context, uid, password, role, operatorUID string) error {
	return v.userStore.UpdateUser(ctx, uid, password, role, operatorUID)
}

func (v *RealSecretVault) DeleteUser(ctx context.Context, uid, operatorUID string) error {
	return v.userStore.DeleteUser(ctx, uid, operatorUID)
}

func (v *RealSecretVault) RecordAdminAudit(ctx context.Context, operatorUID, action, targetUID, detail string) {
	v.adminAudit.Record(ctx, operatorUID, action, targetUID, detail)
}

func (v *RealSecretVault) Close() error {
	return v.encryptor.Close()
}

// DSN 返回 MySQL 连接串。
func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}
