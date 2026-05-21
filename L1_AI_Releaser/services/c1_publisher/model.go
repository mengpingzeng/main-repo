// Package c1_publisher 定义发布记录数据模型。
package c1_publisher

import "time"

// PublishRecord 发布记录数据库模型。
// 对应 publish_record 表。
type PublishRecord struct {
	ID          int64     `json:"id"`
	TaskID      string    `json:"task_id"`
	AccountID   string    `json:"account_id"`
	UID         string    `json:"uid"`
	Platform    string    `json:"platform"`
	ContentHash string    `json:"content_hash"`
	Status      string    `json:"status"`      // "ok" | "fail"
	PostID      string    `json:"post_id"`
	ErrorCode   string    `json:"error_code"`
	ErrorMsg    string    `json:"error_msg"`
	NovelName   string    `json:"novel_name"`   // 作品名（番茄小说）
	SkillID     string    `json:"skill_id"`    // 可选，Skill 标识
	SessionID   string    `json:"session_id"`  // 可选，会话标识
	CalledAt    time.Time `json:"called_at"`
	PublishedAt time.Time `json:"publishedAt"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewPublishRecord 从 PublishResult 创建发布记录。
func NewPublishRecord(taskID string, result PublishResult, skillID, sessionID, novelName string) *PublishRecord {
	now := time.Now().UTC()
	return &PublishRecord{
		TaskID:    taskID,
		AccountID: result.AccountID,
		UID:       result.UID,
		Platform:  result.Platform,
		Status:    result.Status,
		PostID:    result.PostID,
		ErrorCode: result.ErrorCode,
		ErrorMsg:  result.ErrorMessage,
		NovelName: novelName,
		SkillID:   skillID,
		SessionID: sessionID,
		CalledAt:  now,
		PublishedAt: now,
		UpdatedAt: now,
	}
}
