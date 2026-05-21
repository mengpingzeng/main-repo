// Package c1_publisher 提供各平台 StatsAdapter 实现。
package c1_publisher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// XhsStatsAdapter 小红书 StatsAdapter。
type XhsStatsAdapter struct {
	client  *http.Client
	baseURL string
}

func NewXhsStatsAdapter(cfg AdapterConfig) *XhsStatsAdapter {
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 10 * time.Second
	}
	return &XhsStatsAdapter{
		client:  &http.Client{Timeout: cfg.RequestTimeout},
		baseURL: cfg.BaseURL,
	}
}

func (a *XhsStatsAdapter) Platform() string { return "xhs" }

func (a *XhsStatsAdapter) FetchStats(ctx context.Context, postID string, credentials string) (*Stats, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/api/stats/note/%s", a.baseURL, postID), nil)
	req.Header.Set("Cookie", credentials)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xhs stats fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("xhs stats returned %d", resp.StatusCode)
	}
	return &Stats{Views: 0, Likes: 0, Comments: 0, Shares: 0}, nil
}

// WechatStatsAdapter 公众号 StatsAdapter。
type WechatStatsAdapter struct {
	client  *http.Client
	baseURL string
}

func NewWechatStatsAdapter(cfg AdapterConfig) *WechatStatsAdapter {
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 10 * time.Second
	}
	return &WechatStatsAdapter{
		client:  &http.Client{Timeout: cfg.RequestTimeout},
		baseURL: cfg.BaseURL,
	}
}

func (a *WechatStatsAdapter) Platform() string { return "wechat" }

func (a *WechatStatsAdapter) FetchStats(ctx context.Context, postID string, credentials string) (*Stats, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/cgi-bin/article/stats?media_id=%s&access_token=%s", a.baseURL, postID, credentials), nil)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wechat stats fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("wechat stats returned %d", resp.StatusCode)
	}
	return &Stats{Views: 0, Likes: 0, Comments: 0, Shares: 0}, nil
}

// FanqieStatsAdapter 番茄小说 StatsAdapter。
// 番茄小说无公开 API，通过 Puppeteer 脚本 fetch_fanqie_stats.js 抓取数据中心数据。
type FanqieStatsAdapter struct {
	novelName  string
	scriptPath string
	nodeBin    string
}

func NewFanqieStatsAdapter(novelName, scriptPath string) *FanqieStatsAdapter {
	return &FanqieStatsAdapter{
		novelName:  novelName,
		scriptPath: scriptPath,
		nodeBin:    "node",
	}
}

func (a *FanqieStatsAdapter) Platform() string { return "fanqie" }

func (a *FanqieStatsAdapter) FetchStats(ctx context.Context, postID string, credentials string) (*Stats, error) {
	input, _ := json.Marshal(map[string]string{"novelName": a.novelName})

	cmd := exec.CommandContext(ctx, a.nodeBin, a.scriptPath, string(input))
	cmd.Env = append(os.Environ(), "FANQIE_COOKIE="+credentials)

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("fanqie stats: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("fanqie stats: %w", err)
	}

	var result struct {
		Success        bool   `json:"success"`
		Error          string `json:"error"`
		Views          int64  `json:"views"`
		Likes          int64  `json:"likes"`
		Comments       int64  `json:"comments"`
		Shares         int64  `json:"shares"`
		CurrentReaders int64  `json:"current_readers"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("fanqie stats parse: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("fanqie stats: %s", result.Error)
	}

	return &Stats{
		Views:    result.Views,
		Likes:    result.Likes,
		Comments: result.Comments,
		Shares:   result.Shares,
	}, nil
}

// ZhulangStatsAdapter 逐浪网 StatsAdapter。
// 逐浪网无公开 API，通过 Puppeteer 脚本抓取数据中心数据。
type ZhulangStatsAdapter struct {
	novelName  string
	scriptPath string
	nodeBin    string
}

func NewZhulangStatsAdapter(novelName, scriptPath string) *ZhulangStatsAdapter {
	return &ZhulangStatsAdapter{
		novelName:  novelName,
		scriptPath: scriptPath,
		nodeBin:    "node",
	}
}

func (a *ZhulangStatsAdapter) Platform() string { return "zhulang" }

func (a *ZhulangStatsAdapter) FetchStats(ctx context.Context, postID string, credentials string) (*Stats, error) {
	input, _ := json.Marshal(map[string]string{"novelName": a.novelName})

	cmd := exec.CommandContext(ctx, a.nodeBin, a.scriptPath, string(input))
	cmd.Env = append(os.Environ(), "ZHULANG_COOKIE="+credentials)

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("zhulang stats: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("zhulang stats: %w", err)
	}

	var result struct {
		Success  bool   `json:"success"`
		Error    string `json:"error"`
		Views    int64  `json:"views"`
		Likes    int64  `json:"likes"`
		Comments int64  `json:"comments"`
		Shares   int64  `json:"shares"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("zhulang stats parse: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("zhulang stats: %s", result.Error)
	}

	return &Stats{
		Views:    result.Views,
		Likes:    result.Likes,
		Comments: result.Comments,
		Shares:   result.Shares,
	}, nil
}
