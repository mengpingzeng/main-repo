// Package c1_publisher 提供微信公众号 PublishAdapter 实现。
package c1_publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WechatPublishAdapter 微信公众号发布适配器。
type WechatPublishAdapter struct {
	client  *http.Client
	baseURL string
}

func NewWechatPublishAdapter(cfg AdapterConfig) *WechatPublishAdapter {
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 30 * time.Second
	}
	return &WechatPublishAdapter{
		client:  &http.Client{Timeout: cfg.RequestTimeout},
		baseURL: cfg.BaseURL,
	}
}

func (a *WechatPublishAdapter) Platform() string {
	return "wechat"
}

func (a *WechatPublishAdapter) CheckInput(product ProductContent) string {
	if product.HTML == "" {
		return "wechat: html is empty"
	}
	return ""
}

func (a *WechatPublishAdapter) Publish(ctx context.Context, product ProductContent, credentials string, maskedDisplay string) *PublishResult {
	var cred struct {
		AppID       string `json:"app_id"`
		AppSecret   string `json:"app_secret"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal([]byte(credentials), &cred); err != nil {
		return &PublishResult{
			Platform: "wechat", Status: "fail",
			ErrorCode: ErrCodeCredentialFailed,
			ErrorMessage: "wechat credentials parse failed",
			MaskedDisplay: maskedDisplay,
		}
	}
	if cred.AccessToken == "" {
		return &PublishResult{
			Platform: "wechat", Status: "fail",
			ErrorCode: ErrCodeCredentialFailed,
			ErrorMessage: "wechat access_token is empty",
			MaskedDisplay: maskedDisplay,
		}
	}

	url := fmt.Sprintf("%s/cgi-bin/draft/add?access_token=%s", a.baseURL, cred.AccessToken)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return &PublishResult{
			Platform: "wechat", Status: "fail",
			ErrorCode: ErrCodeBuildRequest,
			ErrorMessage: "create http request failed",
			MaskedDisplay: maskedDisplay,
		}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		result := classifyHTTPError("wechat", err)
		result.MaskedDisplay = maskedDisplay
		return result
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		return &PublishResult{
			Platform: "wechat",
			Status:   "ok",
			PostID:   fmt.Sprintf("media_%d", time.Now().UnixNano()%100000),
			MaskedDisplay: maskedDisplay,
		}
	case 401, 403:
		return &PublishResult{
			Platform: "wechat", Status: "fail",
			ErrorCode: ErrCodeAccount401,
			ErrorMessage: "wechat access_token invalid",
			MaskedDisplay: maskedDisplay,
		}
	case 429:
		return &PublishResult{
			Platform: "wechat", Status: "fail",
			ErrorCode: ErrCodePlatformRateLimited,
			ErrorMessage: "wechat rate limited",
			MaskedDisplay: maskedDisplay,
		}
	default:
		return &PublishResult{
			Platform: "wechat", Status: "fail",
			ErrorCode: ErrCodePlatformError,
			ErrorMessage: fmt.Sprintf("wechat unexpected status %d", resp.StatusCode),
			MaskedDisplay: maskedDisplay,
		}
	}
}
