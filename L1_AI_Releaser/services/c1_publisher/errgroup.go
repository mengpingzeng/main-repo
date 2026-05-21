// Package c1_publisher 提供最小 errgroup 实现。
// 替代 golang.org/x/sync/errgroup（标准库版本）。
package c1_publisher

import (
	"context"
	"sync"
)

// errgroup 一组 goroutine 的并发控制。
type errgroup struct {
	cancel  func()
	wg      sync.WaitGroup
	errOnce sync.Once
	err     error
}

// withContext 创建带 context 的 errgroup。
func withContext(ctx context.Context) (*errgroup, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	return &errgroup{cancel: cancel}, ctx
}

// goWait 启动一个 goroutine。
func (g *errgroup) goWait(f func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := f(); err != nil {
			g.errOnce.Do(func() {
				g.err = err
				if g.cancel != nil {
					g.cancel()
				}
			})
		}
	}()
}

// wait 等待所有 goroutine 完成。
func (g *errgroup) wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	return g.err
}
