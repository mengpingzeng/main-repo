package workflow_engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"clawstudios/pkg/logging"
)

// C1Publisher 发布接口（L1 C1 模块实现）
type C1Publisher interface {
	Publish(ctx context.Context, products map[string]ProductContent, accounts []AccountRef) ([]PublishResult, error)
}

// MDWriter 写 MD 档案接口（L1 A4 模块实现）
type MDWriter interface {
	WriteMD(ctx context.Context, req MDWriteRequest) (mdPath string, err error)
}

// DraftFetcher 草稿读取接口（OSS 实现）
type DraftFetcher interface {
	Fetch(ctx context.Context, taskID string, sessionID string, version int) (draftText string, err error)
}

// EngineConfig 引擎配置
type EngineConfig struct {
	DB           *sql.DB
	C1Publisher  C1Publisher
	MDWriter     MDWriter
	DraftFetcher DraftFetcher
	WSHub        *WSHub
}

// ==================== Mock 实现 ====================

type MockC1Publisher struct {
	Results []PublishResult
	Err     error
}

func (m *MockC1Publisher) Publish(ctx context.Context, products map[string]ProductContent, accounts []AccountRef) ([]PublishResult, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Results != nil {
		return m.Results, nil
	}
	var results []PublishResult
	for _, acc := range accounts {
		results = append(results, PublishResult{
			AccountID: acc.AccountID,
			Platform:  acc.Platform,
			Status:    "ok",
			PostID:    "mock_" + acc.Platform + "_1",
		})
	}
	return results, nil
}

type MockMDWriter struct {
	MDPath string
	Err    error
}

func (m *MockMDWriter) WriteMD(ctx context.Context, req MDWriteRequest) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	if m.MDPath != "" {
		return m.MDPath, nil
	}
	return "tasks/" + time.Now().Format("2006/01") + "/" + req.TaskID + ".md", nil
}

type MockDraftFetcher struct {
	Draft string
	Err   error
}

func (m *MockDraftFetcher) Fetch(ctx context.Context, taskID string, sessionID string, version int) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	return m.Draft, nil
}

// SMDraftFetcher 从 Session Manager 的数据目录读取 draft
type SMDraftFetcher struct {
	DataDir string
}

type smSessionInfo struct {
	SessionID string `json:"session_id"`
	SkillID   string `json:"skill_id"`
	Model     string `json:"model"`
}

type smTask struct {
	Sessions []smSessionInfo `json:"-"`
}

func (f *SMDraftFetcher) FetchTaskMeta(taskID string) (skillID, model string) {
	path := fmt.Sprintf("%s/tasks/%s/sessions.json", f.DataDir, taskID)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	var sessions []smSessionInfo
	if err := json.Unmarshal(data, &sessions); err != nil {
		return "", ""
	}
	if len(sessions) > 0 {
		return sessions[0].SkillID, sessions[0].Model
	}
	return "", ""
}

func (f *SMDraftFetcher) Fetch(ctx context.Context, taskID string, sessionID string, version int) (string, error) {
	path := fmt.Sprintf("%s/tasks/%s/sessions/%s/cwd/current_draft.md", f.DataDir, taskID, sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		// Fallback: try versioned draft in task dir
		path = fmt.Sprintf("%s/tasks/%s/draft_v%d.md", f.DataDir, taskID, version)
		data, err = os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("draft not found: %w", err)
		}
	}
	return string(data), nil
}

// LocalDraftFetcher 从本地文件系统读取草稿
type LocalDraftFetcher struct {
	BasePath string
}

func (l *LocalDraftFetcher) Fetch(ctx context.Context, taskID string, sessionID string, version int) (string, error) {
	path := fmt.Sprintf("%s/tasks/%s/draft_v%d.md", l.BasePath, taskID, version)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("local draft fetch %s: %w", path, err)
	}
	return string(data), nil
}

// ==================== Store helpers ====================

func setStatus(ctx context.Context, db *sql.DB, task *WorkflowTask, status, errMsg string) {
	now := time.Now().UTC()
	task.Status = status
	task.ErrorMsg = errMsg
	if _, err := db.ExecContext(ctx,
		`UPDATE workflow_task SET status=?, error_msg=?, updated_at=? WHERE task_id=?`,
		status, errMsg, now, task.TaskID); err != nil {
		logger := logging.FromContext(ctx)
		if logger != nil {
			logger.Error(logging.ErrDatabaseError, "setStatus(%s -> %s) failed: task=%s err=%v",
				task.TaskID, status, task.TaskID, err)
		}
	}
}

func updateStepProgress(ctx context.Context, db *sql.DB, task *WorkflowTask, step string, retry int) {
	now := time.Now().UTC()
	task.CurrentStep = step
	task.StepRetry = retry
	task.StepUpdatedAt = now
	if _, err := db.ExecContext(ctx,
		`UPDATE workflow_task SET current_step=?, step_retry=?, step_updated_at=? WHERE task_id=?`,
		step, retry, now, task.TaskID); err != nil {
		logger := logging.FromContext(ctx)
		if logger != nil {
			logger.Error(logging.ErrDatabaseError, "updateStepProgress(%s) failed: task=%s step=%s err=%v",
				task.TaskID, task.TaskID, step, err)
		}
	}
}

func setStepResult(ctx context.Context, db *sql.DB, task *WorkflowTask, nextStatus, completedStep string) {
	now := time.Now().UTC()
	task.Status = nextStatus
	task.CurrentStep = completedStep
	task.StepRetry = 0
	if _, err := db.ExecContext(ctx,
		`UPDATE workflow_task SET status=?, current_step=?, step_retry=0, step_updated_at=? WHERE task_id=?`,
		nextStatus, completedStep, now, task.TaskID); err != nil {
		logger := logging.FromContext(ctx)
		if logger != nil {
			logger.Error(logging.ErrDatabaseError, "setStepResult(%s -> %s) failed: task=%s err=%v",
				task.TaskID, nextStatus, task.TaskID, err)
		}
	}
}

func setPublishResults(ctx context.Context, db *sql.DB, task *WorkflowTask, nextStatus string) {
	data, _ := json.Marshal(task.PublishResults)
	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx,
		`UPDATE workflow_task SET status=?, publish_results=?, current_step='publishing', step_retry=0, step_updated_at=? WHERE task_id=?`,
		nextStatus, string(data), now, task.TaskID); err != nil {
		logger := logging.FromContext(ctx)
		if logger != nil {
			logger.Error(logging.ErrDatabaseError, "setPublishResults(%s -> %s) failed: task=%s err=%v",
				task.TaskID, nextStatus, task.TaskID, err)
		}
	}
}

func insertTask(ctx context.Context, db *sql.DB, task *WorkflowTask) error {
	data, _ := json.Marshal(task.PublishResults)
	accountsJSON, _ := json.Marshal(task.Accounts)
	now := time.Now().UTC()
	_, err := db.ExecContext(ctx,
		`INSERT INTO workflow_task
			(task_id, uid, skill_id, topic, novel_name, title, volume_name, chapter_number,
			 platform, status, session_id, draft_version, draft_hash,
			 md_path, trace_id, publish_results, accounts, current_step, step_retry, step_updated_at, error_msg)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,0,?,?)`,
		task.TaskID, task.UID, task.SkillID, task.Topic,
		task.NovelName, task.Title, task.VolumeName, task.ChapterNumber,
		task.Platform, task.Status,
		task.SessionID, task.DraftVersion, task.DraftHash, task.MDPath, task.TraceID,
		string(data), string(accountsJSON), task.CurrentStep, now, task.ErrorMsg)
	return err
}

func loadTask(ctx context.Context, db *sql.DB, taskID string) (*WorkflowTask, error) {
	row := db.QueryRowContext(ctx,
		`SELECT task_id, uid, skill_id, topic, novel_name, title, volume_name, chapter_number,
		        platform, status, session_id, draft_version, draft_hash,
		        md_path, trace_id, publish_results, accounts,
		        current_step, step_retry, step_updated_at, error_msg,
		        created_at, updated_at
		 FROM workflow_task WHERE task_id=?`, taskID)

	var task WorkflowTask
	var resultsJSON string
	var accountsJSON string
	var stepUpdatedAt time.Time
	var createdAt time.Time
	var updatedAt time.Time
	var draftHash sql.NullString
	err := row.Scan(&task.TaskID, &task.UID, &task.SkillID, &task.Topic,
		&task.NovelName, &task.Title, &task.VolumeName, &task.ChapterNumber,
		&task.Platform, &task.Status, &task.SessionID, &task.DraftVersion, &draftHash,
		&task.MDPath, &task.TraceID, &resultsJSON, &accountsJSON,
		&task.CurrentStep, &task.StepRetry, &stepUpdatedAt, &task.ErrorMsg,
		&createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	task.StepUpdatedAt = stepUpdatedAt
	task.CreatedAt = createdAt
	task.UpdatedAt = updatedAt
	if draftHash.Valid {
		task.DraftHash = draftHash.String
	}
	if resultsJSON != "" {
		json.Unmarshal([]byte(resultsJSON), &task.PublishResults)
	}
	if accountsJSON != "" {
		json.Unmarshal([]byte(accountsJSON), &task.Accounts)
	}
	return &task, nil
}
