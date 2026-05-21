// Package c1_publisher 实现 L1 能力层 C1 发布模块。
//
// 职责：接收内容 + 账号列表，并发发布到各平台（小红书/公众号/番茄小说）。
// 部分成功语义：单条目失败不阻塞其他条目。
// 唯一调用方：③ Workflow Engine。
package c1_publisher

import "context"

// Publisher 是 C1 发布模块的唯一对外接口。
// Workflow Engine 通过此接口发起发布任务。
type Publisher interface {
	Publish(ctx context.Context, req PublishRequest) (*PublishResponse, error)
	Health(ctx context.Context) error
	Close() error
}
