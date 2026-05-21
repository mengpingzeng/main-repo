package vault

import (
	"errors"
	"fmt"
	"net/http"
)

// SecretError 是 A1 模块的统一错误类型。
type SecretError struct {
	Code       string
	Message    string
	HTTPStatus int
	Retryable  bool
	Cause      error
}

func (e *SecretError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *SecretError) Unwrap() error {
	return e.Cause
}

// Is 用于 errors.Is 判断。
func (e *SecretError) Is(target error) bool {
	t, ok := target.(*SecretError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

var (
	ErrInvalidInput = &SecretError{
		Code:       "INVALID_INPUT",
		Message:    "invalid input: uid, platform, and credentials are required",
		HTTPStatus: http.StatusBadRequest,
		Retryable:  false,
	}

	ErrAccountNotFound = &SecretError{
		Code:       "ACCOUNT_NOT_FOUND",
		Message:    "account not found",
		HTTPStatus: http.StatusNotFound,
		Retryable:  false,
	}

	ErrUnauthorized = &SecretError{
		Code:       "UNAUTHORIZED",
		Message:    "caller not authorized for this operation",
		HTTPStatus: http.StatusForbidden,
		Retryable:  false,
	}

	ErrKMSUnavailable = &SecretError{
		Code:       "KMS_UNAVAILABLE",
		Message:    "KMS service unavailable, retry later",
		HTTPStatus: http.StatusServiceUnavailable,
		Retryable:  true,
	}

	ErrDecryptFailed = &SecretError{
		Code:       "DECRYPT_FAILED",
		Message:    "decryption failed, data may be corrupted",
		HTTPStatus: http.StatusInternalServerError,
		Retryable:  false,
	}

	ErrDBFailure = &SecretError{
		Code:       "DB_FAILURE",
		Message:    "database operation failed",
		HTTPStatus: http.StatusInternalServerError,
		Retryable:  false,
	}

	ErrUserExists = &SecretError{
		Code:       "USER_EXISTS",
		Message:    "username already exists",
		HTTPStatus: http.StatusConflict,
		Retryable:  false,
	}

	ErrInvalidCredentials = &SecretError{
		Code:       "INVALID_CREDENTIALS",
		Message:    "invalid username or password",
		HTTPStatus: http.StatusUnauthorized,
		Retryable:  false,
	}

	ErrInvalidToken = &SecretError{
		Code:       "INVALID_TOKEN",
		Message:    "invalid or expired token",
		HTTPStatus: http.StatusUnauthorized,
		Retryable:  false,
	}

	ErrHasAccounts = &SecretError{
		Code:       "HAS_ACCOUNTS",
		Message:    "用户还有已绑定账号，请先解绑",
		HTTPStatus: http.StatusConflict,
		Retryable:  false,
	}

	ErrCannotDeleteSelf = &SecretError{
		Code:       "FORBIDDEN",
		Message:    "不能删除自己",
		HTTPStatus: http.StatusForbidden,
		Retryable:  false,
	}

	ErrCannotChangeOwnRole = &SecretError{
		Code:       "FORBIDDEN",
		Message:    "不能修改自己的角色",
		HTTPStatus: http.StatusForbidden,
		Retryable:  false,
	}

	ErrPasswordTooShort = &SecretError{
		Code:       "INVALID_PASSWORD",
		Message:    "密码至少需要 8 位",
		HTTPStatus: http.StatusBadRequest,
		Retryable:  false,
	}

	ErrForbidden = &SecretError{
		Code:       "FORBIDDEN",
		Message:    "admin role required",
		HTTPStatus: http.StatusForbidden,
		Retryable:  false,
	}

	ErrTokenStale = &SecretError{
		Code:       "TOKEN_STALE",
		Message:    "密码已修改，请重新登录",
		HTTPStatus: http.StatusUnauthorized,
		Retryable:  false,
	}

	// ErrPlatformNotSupported 该平台暂不支持 Cookie 健康检测。
	ErrPlatformNotSupported = &SecretError{
		Code:       "PLATFORM_NOT_SUPPORTED",
		Message:    "this platform does not support cookie health check yet",
		HTTPStatus: http.StatusUnprocessableEntity,
		Retryable:  false,
	}
)

// WrapError 包装底层错误，保留错误码。
func WrapError(base *SecretError, cause error) *SecretError {
	return &SecretError{
		Code:       base.Code,
		Message:    base.Message,
		HTTPStatus: base.HTTPStatus,
		Retryable:  base.Retryable,
		Cause:      cause,
	}
}

// ErrorCode 从 error 中提取错误码，用于审计日志。
func ErrorCode(err error) string {
	var se *SecretError
	if errors.As(err, &se) {
		return se.Code
	}
	return "UNKNOWN"
}

// IsRetryable 判断错误是否可重试。
func IsRetryable(err error) bool {
	var se *SecretError
	if errors.As(err, &se) {
		return se.Retryable
	}
	return false
}

// HTTPStatusCode 从 error 中提取 HTTP 状态码。
func HTTPStatusCode(err error) int {
	var se *SecretError
	if errors.As(err, &se) {
		return se.HTTPStatus
	}
	return http.StatusInternalServerError
}
