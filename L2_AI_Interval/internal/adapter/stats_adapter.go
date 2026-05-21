package adapter

import (
	"context"
	"fmt"
)

// StatsAdapter 与 v2_4 文档模块⑥ C1 发布的 FetchStats 方法严格对齐
// 调度器不直连平台，所有平台数据必须通过此接口获取
type StatsAdapter interface {
	Fetch(ctx context.Context, postID string, platform string) (*PlatformStats, error)
}

type PlatformStats struct {
	PostID   string `json:"post_id"`
	Platform string `json:"platform"`
	Views    int64  `json:"views"`
	Likes    int64  `json:"likes"`
	Comments int64  `json:"comments"`
	Shares   int64  `json:"shares"`
}

// FetchError 错误分类（C1 StatsAdapter 返回时带类型标记）
type FetchError struct {
	Type     string // timeout / not_found / rate_limited / platform_error / network
	PostID   string
	Platform string
	Err      error
}

func (e *FetchError) Error() string {
	return fmt.Sprintf("fetch %s/%s [%s]: %v", e.Platform, e.PostID, e.Type, e.Err)
}

func (e *FetchError) Unwrap() error {
	return e.Err
}
