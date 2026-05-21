package vault

import "time"

// AccountCredential 账号凭证数据库模型。
type AccountCredential struct {
	AccountID     string    `db:"account_id"`
	UID           string    `db:"uid"`
	Platform      string    `db:"platform"`
	Credential    string    `db:"credential"`
	MaskedDisplay string    `db:"masked_display"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

// User 用户模型。
type User struct {
	UID               string     `db:"uid"`
	Username          string     `db:"username"`
	Password          string     `db:"password"`
	Role              string     `db:"role"`
	PasswordChangedAt *time.Time `db:"password_changed_at"`
	CreatedAt         time.Time  `db:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at"`
}

// AuditEntry 审计日志条目。
type AuditEntry struct {
	ID        int64     `db:"id"`
	AccountID string    `db:"account_id"`
	Action    string    `db:"action"`
	Caller    string    `db:"caller"`
	Result    string    `db:"result"`
	ErrorCode string    `db:"error_code"`
	CreatedAt time.Time `db:"created_at"`
}
