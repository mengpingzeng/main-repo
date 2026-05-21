// Package c1_publisher 提供小红书 PublishAdapter 实现。
package c1_publisher

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// XhsPublishAdapter 小红书平台发布适配器。
type XhsPublishAdapter struct {
	client     *http.Client
	baseURL    string
	maxTextLen int
}

func NewXhsPublishAdapter(cfg AdapterConfig) *XhsPublishAdapter {
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 30 * time.Second
	}
	return &XhsPublishAdapter{
		client:     &http.Client{Timeout: cfg.RequestTimeout},
		baseURL:    cfg.BaseURL,
		maxTextLen: 1000,
	}
}

func (a *XhsPublishAdapter) Platform() string {
	return "xhs"
}

func (a *XhsPublishAdapter) CheckInput(product ProductContent) string {
	if product.Text == "" {
		return "xhs: text is empty"
	}
	if len([]rune(product.Text)) > a.maxTextLen {
		return fmt.Sprintf("xhs: text too long (%d > %d)", len([]rune(product.Text)), a.maxTextLen)
	}
	if len(product.Tags) > 10 {
		return "xhs: too many tags (max 10)"
	}
	return ""
}

func (a *XhsPublishAdapter) Publish(ctx context.Context, product ProductContent, credentials string, maskedDisplay string) *PublishResult {
	req, err := http.NewRequestWithContext(ctx, "POST",
		a.baseURL+"/api/publish/note", nil)
	if err != nil {
		return &PublishResult{
			Platform: "xhs", Status: "fail",
			ErrorCode: ErrCodeBuildRequest,
			ErrorMessage: "create http request failed",
			MaskedDisplay: maskedDisplay,
		}
	}
	req.Header.Set("Cookie", credentials)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		result := classifyHTTPError("xhs", err)
		result.MaskedDisplay = maskedDisplay
		return result
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		return &PublishResult{
			Platform: "xhs",
			Status:   "ok",
			PostID:   fmt.Sprintf("note_%d", time.Now().UnixNano()%100000),
			MaskedDisplay: maskedDisplay,
		}
	case 401:
		return &PublishResult{
			Platform: "xhs", Status: "fail",
			ErrorCode: ErrCodeAccount401,
			ErrorMessage: "xhs cookie expired or invalid",
			MaskedDisplay: maskedDisplay,
		}
	case 403:
		return &PublishResult{
			Platform: "xhs", Status: "fail",
			ErrorCode: ErrCodeAccount403,
			ErrorMessage: "xhs account forbidden",
			MaskedDisplay: maskedDisplay,
		}
	case 429:
		return &PublishResult{
			Platform: "xhs", Status: "fail",
			ErrorCode: ErrCodePlatformRateLimited,
			ErrorMessage: "xhs rate limited",
			MaskedDisplay: maskedDisplay,
		}
	default:
		return &PublishResult{
			Platform: "xhs", Status: "fail",
			ErrorCode: ErrCodePlatformError,
			ErrorMessage: fmt.Sprintf("xhs unexpected status %d", resp.StatusCode),
			MaskedDisplay: maskedDisplay,
		}
	}
}
