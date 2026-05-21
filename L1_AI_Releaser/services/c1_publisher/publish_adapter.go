// Package c1_publisher 定义平台发布适配器接口。
package c1_publisher

import "context"

// PublishAdapter 封装单个平台的发布 API。
// 一个平台 = 一个 Adapter 实现。
// 新增平台只需新增一个 Adapter 实现，C1 主逻辑不动。
type PublishAdapter interface {
	// Publish 发布一条内容到平台。
	// credentials: 平台凭证明文（来自 A1），Adapter 按需解析（cookie/JSON token）。
	Publish(ctx context.Context, product ProductContent, credentials string, maskedDisplay string) *PublishResult

	// CheckInput 发布前校验内容合规性。返回 "" 通过。
	CheckInput(product ProductContent) string

	// Platform 返回平台标识字符串。
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
// 由 ④ 定时调度器调用。
type StatsAdapter interface {
	FetchStats(ctx context.Context, postID string, credentials string) (*Stats, error)
	Platform() string
}
