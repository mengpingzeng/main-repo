// Package v1 定义 C1 发布模块的类型契约。
package v1

// PublishRequest 一次发布任务的完整输入。
// 由 Workflow Engine 构造。
type PublishRequest struct {
	TaskID   string            // 任务唯一标识，格式 task_xxxxxxxx
	Products map[string]ProductContent // 各平台产物映射 key=platform
	Accounts []AccountRef      // 要发布的账号列表
	TraceID  string            // 链路追踪 ID
	SkillID  string            // 可选，标识本次发布用了哪个 Skill
	SessionID string           // 可选，标识本次发布属于哪一轮对话
}

// ProductContent 单个平台的产物内容。
type ProductContent struct {
	Text string   // 正文内容（小红书用）
	HTML string   // 富文本内容（公众号用）
	Tags []string // 标签/话题列表
}

// AccountRef 发布目标账号引用。
type AccountRef struct {
	AccountID string // 账号唯一标识，格式 acc_xxxxxxxx
	UID       string // 用户唯一标识，用于调 A1 做归属校验
	Platform  string // 平台标识："xhs" | "wechat" | "fanqie" | "douyin" | "zhulang"
}

// PublishResponse 发布任务的结果。
type PublishResponse struct {
	TaskID  string          // 任务 ID（回显）
	Results []PublishResult // 每个账号平台的发布结果
	Summary PublishSummary  // 汇总信息
}

// PublishResult 单个账号平台的发布结果。
type PublishResult struct {
	AccountID    string // 账号唯一标识
	UID          string // 用户 ID
	Platform     string // 平台标识
	Status       string // "ok" 或 "fail"
	PostID       string // 平台返回的帖子 ID（仅 Status=ok 时有值）
	ErrorCode    string // 失败时的错误码
	ErrorMessage string // 失败时的错误描述（不含凭证信息）
	MaskedDisplay string // 平台账号脱敏标识，用于档案展示
}

// PublishSummary 发布汇总。
type PublishSummary struct {
	Total     int // 总发布条目数
	Succeeded int // 成功数
	Failed    int // 失败数
}
