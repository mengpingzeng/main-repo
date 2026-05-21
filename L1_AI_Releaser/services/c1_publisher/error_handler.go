// Package c1_publisher 提供错误分类工具函数。
package c1_publisher

import (
	"errors"
	"fmt"
	"net"
	"os"
)

// classifyHTTPError 分类 HTTP 层错误（非 HTTP 状态码错误）。
// 区分超时和网络不可达。
func classifyHTTPError(platform string, err error) *PublishResult {
	if isTimeout(err) {
		return &PublishResult{
			Platform:     platform,
			Status:       "fail",
			ErrorCode:    ErrCodeAPITimeout,
			ErrorMessage: fmt.Sprintf("%s api timeout", platform),
		}
	}
	return &PublishResult{
		Platform:     platform,
		Status:       "fail",
		ErrorCode:    ErrCodeNetworkUnreachable,
		ErrorMessage: fmt.Sprintf("%s api unreachable: %v", platform, err),
	}
}

// classifyHTTPStatusCode 按 HTTP 状态码分类错误。
func classifyHTTPStatusCode(platform string, statusCode int, body string) *PublishResult {
	switch statusCode {
	case 401:
		return &PublishResult{
			Platform: platform, Status: "fail",
			ErrorCode: ErrCodeAccount401,
			ErrorMessage: fmt.Sprintf("%s account credentials expired or invalid", platform),
		}
	case 403:
		return &PublishResult{
			Platform: platform, Status: "fail",
			ErrorCode: ErrCodeAccount403,
			ErrorMessage: fmt.Sprintf("%s account forbidden", platform),
		}
	case 429:
		return &PublishResult{
			Platform: platform, Status: "fail",
			ErrorCode: ErrCodePlatformRateLimited,
			ErrorMessage: fmt.Sprintf("%s rate limited", platform),
		}
	case 500, 502, 503, 504:
		return &PublishResult{
			Platform: platform, Status: "fail",
			ErrorCode: ErrCodePlatformError,
			ErrorMessage: fmt.Sprintf("%s server error: %s", platform, body),
		}
	default:
		return &PublishResult{
			Platform: platform, Status: "fail",
			ErrorCode: ErrCodePlatformError,
			ErrorMessage: fmt.Sprintf("%s unexpected status %d", platform, statusCode),
		}
	}
}

// isTimeout 判断错误是否为超时错误。
func isTimeout(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return os.IsTimeout(err)
}
