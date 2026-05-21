// Package v1 定义 PublishAdapter 和 StatsAdapter 契约接口。
package v1

import "context"

// PublishAdapter 封装单个平台的发布 API。
// 一个平台 = 一个 Adapter 实现。
type PublishAdapter interface {
	Publish(ctx context.Context, product ProductContent, credentials string, maskedDisplay string) *PublishResult
	CheckInput(product ProductContent) string
	Platform() string
}

// Stats 平台数据统计结果。
type Stats struct {
	Views    int64
	Likes    int64
	Comments int64
	Shares   int64
}

// StatsAdapter 封装单个平台的数据拉取 API。
// 由定时调度器调用。
type StatsAdapter interface {
	FetchStats(ctx context.Context, postID string, credentials string) (*Stats, error)
	Platform() string
}
