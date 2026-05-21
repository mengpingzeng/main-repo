// Package c1_publisher 提供 Prometheus 指标。
package c1_publisher

import "sync/atomic"

// PublishMetrics 发布操作的指标。
type PublishMetrics struct {
	TotalCalls     int64
	TotalSucceeded int64
	TotalFailed    int64
	TotalUnits     int64
}

var globalMetrics = &PublishMetrics{}

// RecordPublish 记录一次发布任务的结果。
func RecordPublish(succeeded, failed, total int) {
	atomic.AddInt64(&globalMetrics.TotalCalls, 1)
	atomic.AddInt64(&globalMetrics.TotalSucceeded, int64(succeeded))
	atomic.AddInt64(&globalMetrics.TotalFailed, int64(failed))
	atomic.AddInt64(&globalMetrics.TotalUnits, int64(total))
}

// GetMetrics 获取当前指标快照。
func GetMetrics() PublishMetrics {
	return PublishMetrics{
		TotalCalls:     atomic.LoadInt64(&globalMetrics.TotalCalls),
		TotalSucceeded: atomic.LoadInt64(&globalMetrics.TotalSucceeded),
		TotalFailed:    atomic.LoadInt64(&globalMetrics.TotalFailed),
		TotalUnits:     atomic.LoadInt64(&globalMetrics.TotalUnits),
	}
}
