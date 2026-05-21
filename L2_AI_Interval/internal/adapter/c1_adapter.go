//go:build c1

package adapter

import (
	"context"

	c1publisher "xlongxia-mvp/services/c1_publisher"
)

// C1StatsAdapter 包装 C1 发布包的 StatsAdapter 实现
// 调度器通过此适配器调用 C1 的 FetchStats，不直连平台
type C1StatsAdapter struct {
	c1 c1publisher.StatsAdapter
}

func NewC1StatsAdapter(c1 c1publisher.StatsAdapter) *C1StatsAdapter {
	return &C1StatsAdapter{c1: c1}
}

func (a *C1StatsAdapter) Fetch(ctx context.Context, postID string, platform string) (*PlatformStats, error) {
	stats, err := a.c1.Fetch(ctx, postID, platform)
	if err != nil {
		return nil, err
	}
	return &PlatformStats{
		PostID:   stats.PostID,
		Platform: stats.Platform,
		Views:    stats.Views,
		Likes:    stats.Likes,
		Comments: stats.Comments,
		Shares:   stats.Shares,
	}, nil
}
