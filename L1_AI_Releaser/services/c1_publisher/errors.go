// Package c1_publisher 定义 C1 发布模块的错误类型与错误码。
package c1_publisher

import "errors"

// 错误码常量
const (
	// 凭证相关
	ErrCodeA1Unavailable   = "A1_UNAVAILABLE"
	ErrCodeUnauthorized    = "UNAUTHORIZED"
	ErrCodeAccountNotFound = "ACCOUNT_NOT_FOUND"
	ErrCodeDecryptFailed   = "DECRYPT_FAILED"
	ErrCodeKMSUnavailable  = "KMS_UNAVAILABLE"
	ErrCodeMissingUID      = "MISSING_UID"

	// 内容相关
	ErrCodeInputInvalid        = "INPUT_INVALID"
	ErrCodeMissingProduct      = "MISSING_PRODUCT"
	ErrCodeUnsupportedPlatform = "UNSUPPORTED_PLATFORM"

	// 平台 API 相关
	ErrCodeAccount401          = "ACCOUNT_401"
	ErrCodeAccount403          = "ACCOUNT_403"
	ErrCodePlatformRateLimited = "PLATFORM_RATE_LIMITED"
	ErrCodeAPITimeout          = "API_TIMEOUT"
	ErrCodeNetworkUnreachable  = "NETWORK_UNREACHABLE"
	ErrCodePlatformError       = "PLATFORM_ERROR"
	ErrCodeParseResponse       = "PARSE_RESPONSE_FAILED"
	ErrCodeBuildRequest        = "BUILD_REQUEST_FAILED"
	ErrCodeCredentialFailed    = "CREDENTIAL_FAILED"
	ErrCodeInternalPanic       = "INTERNAL_PANIC"

	// 预检相关
	ErrCodePreFlightFailed = "PRE_FLIGHT_FAILED"
)

// 模块级错误
var (
	ErrInvalidInput     = errors.New("c1: invalid input")
	ErrA1Unavailable    = errors.New("c1: A1 service unavailable")
	ErrAllPublishFailed = errors.New("c1: all publish units failed")
)

// IsRetryable 判断错误码是否可重试。
func IsRetryable(errorCode string) bool {
	switch errorCode {
	case ErrCodeA1Unavailable, ErrCodeKMSUnavailable,
		ErrCodePlatformRateLimited, ErrCodeAPITimeout,
		ErrCodeNetworkUnreachable:
		return true
	}
	return false
}

// ShouldNotifyUser 判断错误码是否需要通知用户。
func ShouldNotifyUser(errorCode string) bool {
	switch errorCode {
	case ErrCodeAccount401, ErrCodeAccount403,
		ErrCodeDecryptFailed, ErrCodeAccountNotFound,
		ErrCodeUnauthorized, ErrCodeInputInvalid:
		return true
	}
	return false
}
