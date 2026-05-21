// Package c1_publisher 提供 MockStatsAdapter 实现。
package c1_publisher

import "context"

// MockStatsAdapter 纯内存实现的 StatsAdapter。
type MockStatsAdapter struct {
	platform string
	stats    *Stats
	failMode bool
	failErr  error
}

func NewMockStatsAdapter(platform string) *MockStatsAdapter {
	return &MockStatsAdapter{
		platform: platform,
		stats: &Stats{
			Views:    100,
			Likes:    20,
			Comments: 5,
			Shares:   3,
		},
	}
}

func (m *MockStatsAdapter) Platform() string { return m.platform }

func (m *MockStatsAdapter) FetchStats(_ context.Context, _ string, _ string) (*Stats, error) {
	if m.failMode {
		return nil, m.failErr
	}
	return m.stats, nil
}

func (m *MockStatsAdapter) SetStats(s *Stats) { m.stats = s }
func (m *MockStatsAdapter) SetFailMode(err error) {
	m.failMode = true
	m.failErr = err
}
