package workflow_engine

import "time"

// PublishInput BFF 传入的发布请求
// platform 从 BFF 传入 → Engine 写入 workflow_task → 发布时从表里读
type PublishInput struct {
	SessionID     string       `json:"sessionId"`
	DraftVersion  int          `json:"draftVersion"`
	Platform      string       `json:"platform"`
	Accounts      []AccountRef `json:"accounts"`
	SkillID       string       `json:"skillId,omitempty"`
	Topic         string       `json:"topic,omitempty"`
	UID           string       `json:"uid,omitempty"`
	TraceID       string       `json:"traceId,omitempty"`
	TaskID        string       `json:"taskId,omitempty"`
	NovelName     string       `json:"novelName,omitempty"`
	Title         string       `json:"title,omitempty"`
	VolumeName    string       `json:"volumeName,omitempty"`
	ChapterNumber int          `json:"chapterNumber,omitempty"`
}

type AccountRef struct {
	AccountID string `json:"accountId"`
	UID       string `json:"uid"`
	Platform  string `json:"platform"`
}

type PublishResult struct {
	AccountID     string `json:"accountId"`
	Platform      string `json:"platform"`
	Status        string `json:"status"` // "ok" | "fail"
	PostID        string `json:"postId"`
	ErrorCode     string `json:"errorCode"`
	MaskedDisplay string `json:"maskedDisplay"`
}

type ProductContent struct {
	Text          string
	HTML          string
	Tags          []string
	NovelName     string
	Title         string
	VolumeName    string
	ChapterNumber int
}

// SessionInfo 对应 A4 模块的 session 摘要
type SessionInfo struct {
	SessionID    string           `json:"sessionId"`
	StartedAt    time.Time        `json:"startedAt"`
	EndedAt      time.Time        `json:"endedAt"`
	MessageCount int              `json:"messageCount"`
	Episodes     []EpisodeSummary `json:"episodes"`
	DraftVersion int              `json:"draftVersion"`
}

// EpisodeSummary 对应 A4 模块的 episode 摘要
type EpisodeSummary struct {
	EpisodeID  string `json:"episodeId"`
	EpochNo    int    `json:"epochNo"`
	UserIntent string `json:"userIntent"`
	Decisions  string `json:"decisions"`
	SummaryOSS string `json:"summaryOss"`
}

// Products 对应 A4 模块的产物内容
type Products struct {
	XhsText      string            `json:"xhsText"`
	WechatHTML   string            `json:"wechatHtml"`
	FanqieNovel  string            `json:"fanqieNovel"`
	ZhulangNovel string            `json:"zhulangNovel"`
	Others       map[string]string `json:"others"`
}

type WorkflowTask struct {
	TaskID         string          `json:"taskId"`
	UID            string          `json:"uid"`
	SkillID        string          `json:"skillId"`
	Topic          string          `json:"topic"`
	Platform       string          `json:"platform"`
	Status         string          `json:"status"`
	SessionID      string          `json:"sessionId"`
	DraftVersion   int             `json:"draftVersion"`
	MDPath         string          `json:"mdPath"`
	TraceID        string          `json:"traceId"`
	PublishResults []PublishResult `json:"publishResults"`
	Accounts       []AccountRef    `json:"accounts"`
	CurrentStep    string          `json:"currentStep"`
	StepRetry      int             `json:"stepRetry"`
	StepUpdatedAt  time.Time       `json:"stepUpdatedAt"`
	ErrorMsg       string          `json:"errorMsg"`
	NovelName      string          `json:"novelName"`
	Title          string          `json:"title"`
	VolumeName     string          `json:"volumeName"`
	ChapterNumber  int             `json:"chapterNumber"`
}

type WSEvent struct {
	TaskID   string `json:"taskId"`
	Stage    string `json:"stage"`
	Status   string `json:"status"`
	Progress string `json:"progress"`
	Err      string `json:"err,omitempty"`
}

type MDWriteRequest struct {
	TaskID         string          `json:"taskId"`
	UID            string          `json:"uid"`
	SkillID        string          `json:"skillId"`
	SkillName      string          `json:"skillName"`
	SkillVersion   string          `json:"skillVersion"`
	Model          string          `json:"model"`
	Topic          string          `json:"topic"`
	Platform       string          `json:"platform"`
	SessionID      string          `json:"sessionId"`
	Sessions       []SessionInfo   `json:"sessions"`
	DraftVersion   int             `json:"draftVersion"`
	PublishResults []PublishResult `json:"publishResults"`
	Products       Products        `json:"products"`
	EpisodeIDs     []string        `json:"episodeIds"`
	TraceID        string          `json:"traceId"`
	CreatedAt      time.Time       `json:"createdAt"`
}

const (
	StatusInit        = "init"
	StatusFetchDraft  = "fetch_draft"
	StatusPublishing  = "publishing"
	StatusPublished   = "published"
	StatusMDWriting   = "md_writing"
	StatusMDWritten   = "md_written"
	StatusDone        = "done"
	StatusDonePartial = "done_partial"
	StatusFailedGen   = "failed_gen"
	StatusFailedMD    = "failed_md"
)

const (
	MaxRetriesFetchDraft = 3
	MaxRetriesMDWriting  = 1
)

func IsTerminal(status string) bool {
	switch status {
	case StatusDone, StatusDonePartial, StatusFailedGen, StatusFailedMD:
		return true
	}
	return false
}
