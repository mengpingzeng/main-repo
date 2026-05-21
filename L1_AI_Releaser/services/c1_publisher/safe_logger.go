// Package c1_publisher 提供安全日志功能。
// 强制脱敏：日志字段白名单机制，禁止打印凭证明文。
package c1_publisher

import (
	"log/slog"
	"os"
	"strings"
)

var logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// safeLogKeys 日志字段安全白名单。
// 以下字段可以出现在日志中，其余字段被自动过滤。
var safeLogKeys = map[string]bool{
	"task_id":    true,
	"account_id": true,
	"uid":        true,
	"platform":   true,
	"status":     true,
	"post_id":    true,
	"error_code": true,
	"error_msg":  true,
	"trace_id":   true,
	"elapsed_ms": true,
	"expected":   true,
	"actual":     true,
	"error":      true,
	"panic":      true,
}

// forbiddenSubstrings 包含以下子串的字段名禁止出现在日志中。
var forbiddenSubstrings = []string{
	"credential", "cookie", "token", "secret",
	"key", "plaintext", "cipher", "password",
}

func isSafeLogKey(key string) bool {
	if safeLogKeys[key] {
		return true
	}
	for _, sub := range forbiddenSubstrings {
		if strings.Contains(strings.ToLower(key), sub) {
			return false
		}
	}
	return false
}

// safeLogAttrs 过滤日志键值对，移除敏感字段。
func safeLogAttrs(kvs ...any) []any {
	filtered := make([]any, 0, len(kvs))
	for i := 0; i+1 < len(kvs); i += 2 {
		key, ok := kvs[i].(string)
		if !ok {
			continue
		}
		if isSafeLogKey(key) {
			filtered = append(filtered, kvs[i], kvs[i+1])
		}
	}
	return filtered
}

func logInfo(msg string, kvs ...any) {
	logger.Info(msg, safeLogAttrs(kvs...)...)
}

func logError(msg string, kvs ...any) {
	logger.Error(msg, safeLogAttrs(kvs...)...)
}

func logWarn(msg string, kvs ...any) {
	logger.Warn(msg, safeLogAttrs(kvs...)...)
}

// sanitizeCredential 对凭证明文脱敏（仅调试用，不输出到生产日志）。
func sanitizeCredential(cred string) string {
	if len(cred) <= 8 {
		return "***"
	}
	return cred[:4] + "..." + cred[len(cred)-4:]
}
