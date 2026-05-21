//go:build !c1

package adapter

import (
	"context"
	"fmt"
)

// NewC1StatsAdapter stub：C1 包未就绪时编译通过，运行时报错
// 生产构建请使用 go build -tags c1
func NewC1StatsAdapter(c1 interface{}) *stubC1Adapter {
	return &stubC1Adapter{}
}

type stubC1Adapter struct{}

func (a *stubC1Adapter) Fetch(ctx context.Context, postID string, platform string) (*PlatformStats, error) {
	return nil, fmt.Errorf("C1 adapter stub: rebuild with -tags c1")
}

