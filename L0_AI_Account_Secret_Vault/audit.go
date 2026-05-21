package vault

import (
	"context"
	"database/sql"
	"time"
)

// AuditWriter 审计日志写入器。
type AuditWriter struct {
	db      *sql.DB
	enabled bool
}

func NewAuditWriter(db *sql.DB, enabled bool) *AuditWriter {
	return &AuditWriter{db: db, enabled: enabled}
}

// Record 记录审计事件。
// 异步写入，不阻塞主流程。
func (w *AuditWriter) Record(ctx context.Context, entry AuditEntry) {
	if !w.enabled {
		return
	}

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}

	go func() {
		bgCtx := context.Background()

		query := `
			INSERT INTO credential_audit_log
				(account_id, action, caller, result, error_code, created_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`

		_, err := w.db.ExecContext(bgCtx, query,
			entry.AccountID,
			entry.Action,
			entry.Caller,
			entry.Result,
			nullIfEmpty(entry.ErrorCode),
			entry.CreatedAt,
		)
		if err != nil {
			logError("audit_write_failed", err,
				"account_id", entry.AccountID,
				"action", entry.Action,
				"caller", entry.Caller,
			)
		}
	}()
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func logError(event string, err error, kv ...string) {
	_ = event
	_ = err
	_ = kv
}

// AdminAuditWriter 管理员操作审计日志写入器。
type AdminAuditWriter struct {
	db      *sql.DB
	enabled bool
}

func NewAdminAuditWriter(db *sql.DB, enabled bool) *AdminAuditWriter {
	return &AdminAuditWriter{db: db, enabled: enabled}
}

func (w *AdminAuditWriter) Record(ctx context.Context, operatorUID, action, targetUID, detail string) {
	if !w.enabled {
		return
	}

	go func() {
		bgCtx := context.Background()
		query := `INSERT INTO admin_audit_log (operator, action, target_uid, detail) VALUES (?, ?, ?, ?)`
		_, err := w.db.ExecContext(bgCtx, query, operatorUID, action, targetUID, detail)
		if err != nil {
			logError("admin_audit_write_failed", err,
				"operator", operatorUID,
				"action", action,
				"target", targetUID,
			)
		}
	}()
}

// MockAuditWriter 内存审计日志写入器（Mock 模式用）。
type MockAuditWriter struct {
	enabled  bool
	auditLog []AuditEntry
}

func NewMockAuditWriter() *MockAuditWriter {
	return &MockAuditWriter{
		enabled:  true,
		auditLog: make([]AuditEntry, 0),
	}
}

func (w *MockAuditWriter) Record(ctx context.Context, entry AuditEntry) {
	if !w.enabled {
		return
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	_ = ctx
	w.auditLog = append(w.auditLog, entry)
}

func (w *MockAuditWriter) GetLogs() []AuditEntry {
	cp := make([]AuditEntry, len(w.auditLog))
	copy(cp, w.auditLog)
	return cp
}
