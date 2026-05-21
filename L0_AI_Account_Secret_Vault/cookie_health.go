package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// fanqieCheckURL 用于检测登录态的番茄小说轻量接口。
// 该接口只需 Cookie 即可调用，不强制要求 msToken / a_bogus 等动态防爬参数。
// 返回 JSON：{"code":0,...} 表示已登录；{"code":-2012,...} 或 code!=0 表示未登录。
const fanqieCheckURL = "https://fanqienovel.com/api/author/account/info/v0/?aid=2503&app_name=muye_novel"

// fanqieProbeTimeout 单次探测超时。
const fanqieProbeTimeout = 8 * time.Second

// fanqieProbeCacheExpiry 服务端内存缓存 TTL，避免同账号短时间内重复打外网。
// 注：客户端 localStorage 已有 1h 缓存，这里是额外的服务端防抖。
const fanqieProbeCacheExpiry = 5 * time.Minute

// probeFanqieLogin 向番茄小说发送带 Cookie 的轻量请求，判断登录态是否有效。
//
// 探测逻辑：
//  1. 以存储的 cookie 字符串为 Cookie Header 发 GET 请求
//  2. HTTP 200 且响应体 JSON code==0 → 登录有效
//  3. HTTP 非 200，或 JSON code!=0，或超时/网络错误 → 登录失效
//
// 注意：退出登录后服务端会立即使 session 失效，即使 cookie 时间戳未过期也会返回 code!= 0。
func probeFanqieLogin(ctx context.Context, cookieStr string) (valid bool, err error) {
	reqCtx, cancel := context.WithTimeout(ctx, fanqieProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, fanqieCheckURL, nil)
	if err != nil {
		return false, fmt.Errorf("build request: %w", err)
	}

	// 设置 Cookie Header
	req.Header.Set("Cookie", cookieStr)
	// 模拟浏览器 UA，避免被 WAF 拦截
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://fanqienovel.com/main/writer/")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	client := &http.Client{
		Timeout: fanqieProbeTimeout,
		// 不自动跟随重定向（302 → 登录页 = session 无效）
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		// 网络错误（DNS 失败、连接超时等）→ 无法判断，返回 error 让上层处理
		return false, fmt.Errorf("probe request failed: %w", err)
	}
	defer resp.Body.Close()

	// 3xx 重定向通常是跳到登录页，说明 session 已失效
	if resp.StatusCode >= 300 {
		return false, nil
	}

	// 非 200 也视为失效
	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	// 解析响应体，检查 code 字段
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return false, fmt.Errorf("read response: %w", err)
	}

	var result struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		// 无法解析 JSON（如返回了 HTML 登录页）→ 失效
		return false, nil
	}

	// code == 0 表示接口调用成功，session 有效
	return result.Code == 0, nil
}

// parseCookieField 从 Cookie 字符串中提取指定名称的 value。
// 支持 URL 编码的 cookie string（如从数据库读出的原始抓取值）。
func parseCookieField(cookieStr, name string) string {
	for _, segment := range strings.Split(cookieStr, ";") {
		segment = strings.TrimSpace(segment)
		idx := strings.IndexByte(segment, '=')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(segment[:idx])
		if key != name {
			continue
		}
		val := strings.TrimSpace(segment[idx+1:])
		if decoded, err := url.QueryUnescape(val); err == nil {
			return decoded
		}
		return val
	}
	return ""
}

// checkPlatformCookieExpiry 按平台分发到对应的检测函数。
//
// 返回：
//   - valid=true, nil            → session 仍然有效
//   - valid=false, nil           → session 已失效（服务端确认）
//   - valid=false, error         → 无法检测（平台未实现 / 网络错误）
func checkPlatformCookieExpiry(ctx context.Context, platform, cookieStr string) (valid bool, err error) {
	switch platform {
	case "fanqie":
		valid, err = probeFanqieLogin(ctx, cookieStr)
		if err != nil {
			// 网络不通时降级：有 sessionid 字段就保守判为有效，避免误报过期
			if parseCookieField(cookieStr, "sessionid") != "" {
				return true, nil
			}
			return false, ErrPlatformNotSupported
		}
		return valid, nil
	default:
		return false, ErrPlatformNotSupported
	}
}
