package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const platformIdentityTimeout = 10 * time.Second

var zhulangWriterPageURL = "https://writer.zhulang.com/book/index.html"

var zhulangUIDPattern = regexp.MustCompile(`uid\s*:\s*"(\d+)"`)

// ResolvePlatformAuthorID 从凭证明文解析平台侧作者唯一标识。
// 番茄：account/info 接口的 mp_name（如 番茄2510925974999303）
// 逐浪：作家专区页面内嵌 uid（如 69108505）
func ResolvePlatformAuthorID(ctx context.Context, platform, credentialsPlaintext string) (string, error) {
	credentialsPlaintext = strings.TrimSpace(credentialsPlaintext)
	if credentialsPlaintext == "" {
		return "", nil
	}

	switch platform {
	case "fanqie":
		return resolveFanqieAuthorID(ctx, credentialsPlaintext)
	case "zhulang":
		return resolveZhulangAuthorID(ctx, credentialsPlaintext)
	default:
		return "", nil
	}
}

func resolveFanqieAuthorID(ctx context.Context, cookieStr string) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, platformIdentityTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, fanqieCheckURL, nil)
	if err != nil {
		return "", fmt.Errorf("build fanqie identity request: %w", err)
	}
	req.Header.Set("Cookie", cookieStr)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://fanqienovel.com/main/writer/")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	client := &http.Client{
		Timeout: platformIdentityTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fanqie identity probe failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 || resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fanqie identity probe http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return "", fmt.Errorf("read fanqie identity response: %w", err)
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			MPName     string `json:"mp_name"`
			AuthorName string `json:"author_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse fanqie identity response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("fanqie identity probe code=%d", result.Code)
	}

	mpName := strings.TrimSpace(result.Data.MPName)
	if mpName != "" {
		return mpName, nil
	}

	// 兜底：部分 Cookie 可能缺 mp_name 字段，尝试 uid_tt
	if uidTT := strings.TrimSpace(parseCookieField(cookieStr, "uid_tt")); uidTT != "" {
		return "uid_tt:" + uidTT, nil
	}

	return "", fmt.Errorf("fanqie identity: mp_name and uid_tt both empty")
}

func resolveZhulangAuthorID(ctx context.Context, cookieStr string) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, platformIdentityTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, zhulangWriterPageURL, nil)
	if err != nil {
		return "", fmt.Errorf("build zhulang identity request: %w", err)
	}
	req.Header.Set("Cookie", cookieStr)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://writer.zhulang.com/")

	client := &http.Client{Timeout: platformIdentityTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("zhulang identity probe failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("zhulang identity probe http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return "", fmt.Errorf("read zhulang identity response: %w", err)
	}

	m := zhulangUIDPattern.FindSubmatch(body)
	if len(m) < 2 {
		return "", fmt.Errorf("zhulang identity: uid not found in writer page")
	}
	return string(m[1]), nil
}
