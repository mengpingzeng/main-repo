// Package c1_publisher 定义配置结构。
package c1_publisher

import (
	"database/sql"
	"time"
)

// Config C1 发布模块的配置。
// PublishStore 由 NewRealPublisher 内部通过 cfg 创建，不在 Config 中暴露。
type Config struct {
	Adapters         []PublishAdapter // 平台 Adapter 列表
	A1BaseURL        string           // A1 凭证服务地址，默认 "http://localhost:8084"
	ConcurrencyLimit int              // 最大并发 goroutine 数，0=不限制
	DB               *sql.DB          // MySQL 连接（nil 时使用内存存储）
}

// AdapterConfig 平台适配器通用配置。
//
// HTTP 模式（小红书、公众号）使用：BaseURL、RequestTimeout
// Puppeteer 模式（番茄小说）使用：ScriptPath、NodeBin、Timeout
// 两种模式的字段互不干扰，零值字段被对应 Adapter 忽略。
type AdapterConfig struct {
	BaseURL        string        // HTTP 模式：平台 API 地址
	RequestTimeout time.Duration // HTTP 模式：请求超时
	MaxRetries     int           // HTTP 模式：最大重试次数（预留）

	ScriptPath string        // Puppeteer 模式：JS 脚本路径
	NodeBin    string        // Puppeteer 模式：node 命令路径
	Timeout    time.Duration // Puppeteer 模式：执行超时
}
