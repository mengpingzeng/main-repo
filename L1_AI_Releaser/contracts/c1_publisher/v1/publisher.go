// Package v1 定义 C1 发布模块的冻结契约。
// 任何实现 packages/services/c1_publisher 的代码必须遵守此契约。
// 契约变更必须走 PR + 所有下游 reviewer 签字。
package v1

import "context"

// Publisher 是 C1 发布模块的唯一对外接口。
// Workflow Engine 通过此接口发起发布任务。
//
// 非功能语义速查：
//
//	Publish: 部分成功语义，单条目失败不阻塞其他，超时分平台看 Adapter 超时
//	Health:  检查 A1 连通性 + DB 连通性
//	Close:   等待在途 goroutine 完成
type Publisher interface {
	Publish(ctx context.Context, req PublishRequest) (*PublishResponse, error)
	Health(ctx context.Context) error
	Close() error
}
