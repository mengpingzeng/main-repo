package a4md

import (
	"context"
	"io"
	"time"
)

// WriteMDInput 首次生成 MD 的输入
// 来源：Workflow Engine（按 §4.5 契约）
type WriteMDInput struct {
	TaskID         string          // 任务 ID，如 task_xxx
	UID            string          // 用户 ID
	Topic          string          // 任务主题
	NovelName      string          // 小说书名
	VolumeName     string          // 分卷名
	Title          string          // 章节标题
	ChapterNumber  int             // 章节号
	CreatedAt      time.Time       // 任务创建时间（ISO8601 UTC）
	SkillID        string          // 使用的 Skill ID（如 xhs_grass_v1）
	SkillName      string          // 使用的 Skill 名称
	SkillVersion   string          // 使用的 Skill 版本
	Model          string          // 使用的 Model（如 deepseek-chat）
	Sessions       []SessionInfo   // 关联的 session 清单
	DraftVersion   int             // 最终选用的 draft 版本号
	Products       Products        // 最终产物内容
	PublishResults []PublishResult // 发布结果
	EpisodeIDs     []string        // episode ID 清单（按 ID 去 OSS 读摘要原文）
	TraceID        string          // 全链路 trace ID
}

// SessionInfo 单个 session 的摘要信息
type SessionInfo struct {
	SessionID    string           // sess_aaa
	StartedAt    time.Time        // 开始时间
	EndedAt      time.Time        // 结束时间
	MessageCount int              // 消息数
	Episodes     []EpisodeSummary // 摘要列表
	DraftVersion int              // 该 session 最终产出的 draft 版本
}

// EpisodeSummary episode 摘要（用于写入"会话历程"段）
type EpisodeSummary struct {
	EpisodeID  string // ep_xxx（格式：ep_{session_id}_{epoch_no}）
	EpochNo    int    // 该 session 内第几段
	UserIntent string // 用户核心诉求
	Decisions  string // Agent 关键决定
	SummaryOSS string // OSS 路径（按需读原文）
}

// Products 最终产物内容
type Products struct {
	XhsText      string            // 小红书文案
	WechatHTML   string            // 公众号 HTML（或长文）
	FanqieNovel  string            // 番茄小说章节内容
	ZhulangNovel string            // 逐浪网小说章节内容
	Others       map[string]string // 未来平台扩展（key=平台名, value=内容）
}

// PublishResult 单次发布结果
type PublishResult struct {
	AccountID     string // 账号 ID
	Platform      string // xhs / wechat / fanqie / zhulang
	Status        string // ok / fail
	PostID        string // 平台返回的 post_id
	PostURL       string // 发布链接
	ErrorCode     string // 平台错误码（status=fail 时有值，如 "TOKEN_EXPIRED"）
	ErrorMsg      string // 失败原因
	MaskedDisplay string // 脱敏展示名（如"用户***123"）
}

// AppendStatsInput 追加写数据反馈的输入
type AppendStatsInput struct {
	TaskID       string     // 目标 task
	StatsPeriod  string     // "24h" | "7d" | "30d"
	Stats        []StatItem // 统计数据行（可为空，空时自动从 MySQL 查）
	DraftVersion int        // 追加目标 draft 版本号
	TraceID      string
}

// StatItem 单条数据反馈
// is_hot 不在模板中出现（文档约束#10），该字段仅用于 C2 看板模块
type StatItem struct {
	AccountID string
	Platform  string
	Views     int
	Likes     int
	Comments  int
	Shares    int
}

// WriteMDResult 首次生成 MD 的返回
type WriteMDResult struct {
	MDPath string // OSS 路径：tasks/{task_id}/archive/draft_v{draft_version}.md
}

// Storage 统一存储接口（附录 D）
// 生产/测试只需切换实现
type Storage interface {
	Put(ctx context.Context, path string, data io.Reader) error
	Get(ctx context.Context, path string) (io.ReadCloser, error)
	Delete(ctx context.Context, path string) error
	List(ctx context.Context, prefix string) ([]string, error)
	Exists(ctx context.Context, path string) (bool, error)
}

// StatsRepository 读取 MySQL platform_stats 表
type StatsRepository interface {
	GetStatsByTaskID(ctx context.Context, taskID string) ([]StatItem, error)
	GetStatsByPeriod(ctx context.Context, taskID string, period string) ([]StatItem, error)
}

// TemplateData 传入 text/template 的顶层数据
type TemplateData struct {
	UID           string
	Topic         string
	NovelName     string
	VolumeName    string
	Title         string
	ChapterNumber int
	TaskID         string
	CreatedAt      string // ISO8601 UTC 格式化后的字符串
	SkillID        string
	SkillName      string
	SkillVersion   string
	Model          string
	DraftVersion   int
	Sessions       []TemplateSession
	Products       TemplateProducts
	PublishResults []TemplatePublishResult
	StatsGroups    []TemplateStatsGroup // nil 表示不输出数据反馈段
}

type TemplateSession struct {
	SessionID    string
	StartedAt    string
	EndedAt      string
	MessageCount int
	DraftVersion int
	Episodes     []TemplateEpisode
}

type TemplateEpisode struct {
	EpisodeID  string
	EpochNo    int
	UserIntent string
	Decisions  string
}

type TemplateProducts struct {
	XhsText      string
	WechatHTML   string
	FanqieNovel  string
	ZhulangNovel string
	Others       map[string]string
}

type TemplatePublishResult struct {
	AccountID     string
	Platform      string
	Status        string
	PostID        string
	PostURL       string
	ErrorCode     string
	MaskedDisplay string
}

type TemplateStatsGroup struct {
	Period string
	Items  []TemplateStatItem
}

type TemplateStatItem struct {
	AccountID string
	Platform  string
	Views     string
	Likes     string
	Comments  string
	Shares    string
}

type noopLogger struct{}

func (n *noopLogger) Warn(msg string, args ...interface{})  {}
func (n *noopLogger) Info(msg string, args ...interface{})  {}
func (n *noopLogger) Error(msg string, args ...interface{}) {}
