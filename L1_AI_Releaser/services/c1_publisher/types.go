// Package c1_publisher 定义 C1 发布模块的核心类型。
package c1_publisher

// PublishRequest 一次发布任务的完整输入。
type PublishRequest struct {
	TaskID    string                    // 任务唯一标识
	Products  map[string]ProductContent // 各平台产物映射 key=platform
	Accounts  []AccountRef              // 要发布的账号列表
	TraceID   string                    // 链路追踪 ID
	SkillID   string                    // 可选，标识本次发布用了哪个 Skill
	SessionID string                    // 可选，标识本次发布属于哪一轮对话
}

// ProductContent 单个平台的产物内容。
type ProductContent struct {
	Text string   // 正文内容
	HTML string   // 富文本内容（公众号用）
	Tags []string // 标签/话题列表（小红书用）

	// 番茄小说 Puppeteer 模式专用字段（omitempty，其他平台忽略）
	NovelName     string `json:"novelName,omitempty"`     // 作品名
	VolumeName    string `json:"volumeName,omitempty"`    // 分卷名，默认"第一卷"
	ChapterNumber int    `json:"chapterNumber,omitempty"` // 章节号，默认 1
	Title         string `json:"title,omitempty"`         // 章节标题，空则取 Text 第一行
}

// AccountRef 发布目标账号引用。
type AccountRef struct {
	AccountID string
	UID       string
	Platform  string
}

// PublishResponse 发布任务的结果。
type PublishResponse struct {
	TaskID  string
	Results []PublishResult
	Summary PublishSummary
}

// PublishResult 单个账号平台的发布结果。
type PublishResult struct {
	AccountID     string
	UID           string
	Platform      string
	Status        string // "ok" | "fail"
	PostID        string // 平台返回的帖子 ID
	ErrorCode     string // 失败时的错误码
	ErrorMessage  string // 失败时的错误描述（不含凭证信息）
	NovelName     string // 作品名（番茄小说专用，透传到 A4 档案）
	MaskedDisplay string // 平台账号脱敏标识，用于档案展示
}

// PublishSummary 发布汇总。
type PublishSummary struct {
	Total     int
	Succeeded int
	Failed    int
}

// PublishUnit 一个发布单元 = 一个账号在一个平台发布一条内容。
type PublishUnit struct {
	TaskID      string
	AccountID   string
	UID         string
	Platform    string
	Product     ProductContent
	Credentials string // 明文凭证（仅在此 goroutine 内可见）
	ContentHash string // SHA-256
	ErrorCode   string // 预检失败的错误码（非空 = 此 unit 无需调平台 API）
	ErrorMsg    string // 预检失败的详细消息
	SkillID     string // 可选，标识本次发布用了哪个 Skill
	SessionID   string // 可选，标识本次发布属于哪一轮对话
	MaskedDisplay string // 平台账号脱敏标识
}
