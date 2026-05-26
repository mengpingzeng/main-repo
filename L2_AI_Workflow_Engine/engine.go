package workflow_engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"clawstudios/pkg/logging"
)

type Engine struct {
	db           *sql.DB
	c1           C1Publisher
	a4           MDWriter
	draftFetcher DraftFetcher
	wsHub        *WSHub
}

func New(cfg EngineConfig) *Engine {
	return &Engine{
		db:           cfg.DB,
		c1:           cfg.C1Publisher,
		a4:           cfg.MDWriter,
		draftFetcher: cfg.DraftFetcher,
		wsHub:        cfg.WSHub,
	}
}

func (e *Engine) Execute(ctx context.Context, input PublishInput) error {
	if err := validateInput(input); err != nil {
		return err
	}

	task, isNew, err := e.upsertTask(ctx, input)
	if err != nil {
		return err
	}

	// Only skip if fully done (not partial/failed)
	if task.Status == StatusDone && !isNew {
		return nil
	}

	// Reset for re-publish of failed/partial tasks
	if IsTerminal(task.Status) && !isNew {
		task.Status = StatusPublishing
		task.PublishResults = nil
		task.CurrentStep = ""
		task.StepRetry = 0
	}
	if IsTerminal(task.Status) && !isNew && task.Status != StatusDonePartial {
		task.Status = StatusPublishing
		task.PublishResults = nil
		task.CurrentStep = ""
		task.StepRetry = 0
	}

	return e.runStateMachine(ctx, task)
}

func (e *Engine) ExecuteAndGetTask(ctx context.Context, input PublishInput) (*WorkflowTask, error) {
	l := logging.FromContext(ctx)

	if err := validateInput(input); err != nil {
		if l != nil {
			l.Error(logging.ErrInvalidParam, "发布参数校验失败: %v", err)
		}
		return nil, err
	}

	if l != nil {
		inJSON, _ := json.Marshal(input)
		l.Info("发布请求入参: %s", string(inJSON))
	}

	task, isNew, err := e.upsertTask(ctx, input)
	if err != nil {
		if l != nil {
			l.Error(logging.ErrDatabaseError, "创建/更新任务失败: %v", err)
		}
		return nil, err
	}

	oldStatus := task.Status

	if task.Status == StatusDone && !isNew {
		if l != nil {
			l.Info("任务已完成,跳过重复发布: task=%s", task.TaskID)
		}
		return task, nil
	}

	if IsTerminal(task.Status) && !isNew && task.Status != StatusDonePartial && task.Status != StatusFailedGen && task.Status != StatusFailedMD && task.Status != StatusPublishedFailed {
		return task, nil
	}

	replayable := task.Status == StatusDonePartial || task.Status == StatusFailedGen || task.Status == StatusFailedMD || task.Status == StatusPublishedFailed
	if task.Status == StatusPublishing && !isNew {
		replayable = true
	}
	if replayable {
		task.Status = StatusInit
		task.PublishResults = nil
		task.CurrentStep = ""
		task.StepRetry = 0
		task.DraftHash = ""
		if l != nil {
			l.Warn(logging.WarnRetryAttempt, "重试发布: task=%s 上次状态=%s 新session=%s", task.TaskID, oldStatus, task.SessionID)
		}
	}

	if l != nil {
		l.Info("开始执行发布流程: task=%s status=%s platform=%s accounts=%d",
			task.TaskID, task.Status, task.Platform, len(task.Accounts))
	}

	if err := e.runStateMachine(ctx, task); err != nil {
		if l != nil {
			l.Error(logging.ErrWorkflowError, "发布流程执行失败: task=%s err=%v", task.TaskID, err)
		}
		return nil, err
	}

	if l != nil {
		resJSON, _ := json.Marshal(task.PublishResults)
		l.Info("发布会流程执行完成: task=%s 最终状态=%s 发布结果=%s",
			task.TaskID, task.Status, string(resJSON))
	}

	return task, nil
}

func (e *Engine) upsertTask(ctx context.Context, input PublishInput) (*WorkflowTask, bool, error) {
	existing, err := loadTask(ctx, e.db, taskIDFromInput(input))
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		updated := false
		if input.NovelName != "" && input.NovelName != existing.NovelName {
			existing.NovelName = input.NovelName
			updated = true
		}
		if input.Title != "" && input.Title != existing.Title {
			existing.Title = input.Title
			updated = true
		}
		if input.VolumeName != "" && input.VolumeName != existing.VolumeName {
			existing.VolumeName = input.VolumeName
			updated = true
		}
		if input.ChapterNumber > 0 && input.ChapterNumber != existing.ChapterNumber {
			existing.ChapterNumber = input.ChapterNumber
			updated = true
		}
		if len(input.Accounts) > 0 {
			existing.Accounts = input.Accounts
			updated = true
		}
		if input.SessionID != "" && input.SessionID != existing.SessionID {
			existing.SessionID = input.SessionID
			existing.DraftHash = ""
			updated = true
		}
		if input.DraftVersion != existing.DraftVersion {
			existing.DraftVersion = input.DraftVersion
			existing.DraftHash = ""
			updated = true
		}
		if updated {
			accountsJSON, _ := json.Marshal(existing.Accounts)
			e.db.ExecContext(ctx,
				`UPDATE workflow_task SET novel_name=?, title=?, volume_name=?, chapter_number=?, accounts=?, session_id=?, draft_version=?, draft_hash=?, updated_at=NOW() WHERE task_id=?`,
				existing.NovelName, existing.Title, existing.VolumeName, existing.ChapterNumber, string(accountsJSON),
				existing.SessionID, existing.DraftVersion, existing.DraftHash, existing.TaskID)
		}
		return existing, false, nil
	}

	task := &WorkflowTask{
		TaskID:        taskIDFromInput(input),
		SessionID:     input.SessionID,
		DraftVersion:  input.DraftVersion,
		Platform:      input.Platform,
		SkillID:       input.SkillID,
		Topic:         input.Topic,
		UID:           input.UID,
		TraceID:       input.TraceID,
		Status:        StatusInit,
		Accounts:      input.Accounts,
		NovelName:     input.NovelName,
		Title:         input.Title,
		VolumeName:    input.VolumeName,
		ChapterNumber: input.ChapterNumber,
	}
	// Use SM task ID as the task ID for draft fetching
	if input.TaskID != "" {
		task.TaskID = input.TaskID
	}
	if err := insertTask(ctx, e.db, task); err != nil {
		return nil, false, err
	}
	return task, true, nil
}

func taskIDFromInput(input PublishInput) string {
	if input.TaskID != "" {
		return input.TaskID
	}
	if input.TraceID != "" {
		return input.TraceID
	}
	return input.SessionID
}

func (e *Engine) runStateMachine(ctx context.Context, task *WorkflowTask) error {
	l := logging.FromContext(ctx)
	var draftContent string

	for !IsTerminal(task.Status) {
		if l != nil {
			l.Info("步骤开始: task=%s status=%s", task.TaskID, task.Status)
		}
		e.pushWS(task, task.Status, "running", "")

		var next string
		var err error

		switch task.Status {
		case StatusInit, StatusFetchDraft:
			if l != nil {
				l.Info("获取草稿: task=%s session=%s draftVersion=%d", task.TaskID, task.SessionID, task.DraftVersion)
			}
			next, draftContent, err = stepFetchDraft(ctx, task, e.draftFetcher, e.db)
			if l != nil {
				if err != nil {
					l.Error(logging.ErrWorkflowError, "获取草稿失败: task=%s err=%v", task.TaskID, err)
				} else {
					l.Info("获取草稿成功: task=%s 草稿长度=%d 下一步=%s", task.TaskID, len(draftContent), next)
				}
			}

		case StatusPublishing:
			if l != nil {
				l.Info("开始发布: task=%s platform=%s 账号数=%d", task.TaskID, task.Platform, len(task.Accounts))
			}
			if draftContent == "" {
				draftContent, err = e.draftFetcher.Fetch(ctx, task.TaskID, task.SessionID, task.DraftVersion)
				if err != nil {
					if l != nil {
						l.Error(logging.ErrWorkflowError, "发布前获取草稿失败: task=%s err=%v", task.TaskID, err)
					}
					return err
				}
			}
			next, err = stepPublishing(ctx, task, e.c1, e.db, draftContent)
			if l != nil {
				if err != nil {
					l.Error(logging.ErrWorkflowError, "发布失败: task=%s err=%v", task.TaskID, err)
				} else {
					resultsJSON, _ := json.Marshal(task.PublishResults)
					l.Info("发布完成: task=%s 结果=%s", task.TaskID, string(resultsJSON))
				}
			}

		case StatusPublished:
			if l != nil {
				l.Info("发布完成,进入文档沉淀: task=%s", task.TaskID)
			}
			next = StatusMDWriting
			setStatus(ctx, e.db, task, next, "")

		case StatusMDWriting:
			if l != nil {
				l.Info("开始文档沉淀: task=%s novel=%s title=%s ch=%d", task.TaskID, task.NovelName, task.Title, task.ChapterNumber)
			}
			if draftContent == "" {
				draftContent, err = e.draftFetcher.Fetch(ctx, task.TaskID, task.SessionID, task.DraftVersion)
				if err != nil {
					if l != nil {
						l.Error(logging.ErrWorkflowError, "文档沉淀前获取草稿失败: task=%s err=%v", task.TaskID, err)
					}
					return err
				}
			}
			if smFetcher, ok := e.draftFetcher.(*SMDraftFetcher); ok {
				skillID, model := smFetcher.FetchTaskMeta(task.TaskID)
				if task.SkillID == "" && skillID != "" {
					task.SkillID = skillID
				}
				if task.Model == "" && model != "" {
					task.Model = model
				}
			}
			next, err = stepMDWriting(ctx, task, e.a4, e.db, draftContent)
			if l != nil {
				if err != nil {
					l.Error(logging.ErrWorkflowError, "文档沉淀失败: task=%s err=%v", task.TaskID, err)
				} else {
					l.Info("文档沉淀完成: task=%s md路径=%s", task.TaskID, task.MDPath)
				}
			}

		case StatusMDWritten:
			next = resolveFinalStatus(task)
			setStatus(ctx, e.db, task, next, "")
			e.pushWS(task, next, "success", "完成")
			if l != nil {
				l.Info("全部流程完成: task=%s 最终状态=%s", task.TaskID, next)
			}

		default:
			if l != nil {
				l.Error(logging.ErrWorkflowError, "未知状态: task=%s status=%s", task.TaskID, task.Status)
			}
			log.Printf("[task=%s] unknown status: %s", task.TaskID, task.Status)
			return fmt.Errorf("unknown status: %s", task.Status)
		}

		if err != nil {
			log.Printf("[task=%s] step error: status=%s, next=%s, err=%v", task.TaskID, task.Status, next, err)
			return err
		}
		task.Status = next
	}
	if l != nil {
		l.Info("状态机结束: task=%s 最终状态=%s", task.TaskID, task.Status)
	}
	log.Printf("[task=%s] state machine finished: status=%s", task.TaskID, task.Status)
	return nil
}

// Replay 重放失败或部分成功的任务
func (e *Engine) Replay(ctx context.Context, taskID string) error {
	l := logging.FromContext(ctx)
	task, err := loadTask(ctx, e.db, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task %s not found", taskID)
	}

	if l != nil {
		l.Info("replay started: task=%s from_status=%s", taskID, task.Status)
	}

	switch task.Status {
	case StatusFailedGen:
		setStatus(ctx, e.db, task, StatusInit, "replay from failed_gen")
		return e.runStateMachine(ctx, task)

	case StatusFailedMD:
		setStatus(ctx, e.db, task, StatusPublished, "replay from failed_md")
		task.StepRetry = 0
		task.MDPath = ""
		updateStepProgress(ctx, e.db, task, "", 0)
		return e.runStateMachine(ctx, task)

	case StatusDonePartial:
		setStatus(ctx, e.db, task, StatusPublished, "replay md from done_partial")
		task.StepRetry = 0
		task.MDPath = ""
		updateStepProgress(ctx, e.db, task, "", 0)
		return e.runStateMachine(ctx, task)

	case StatusPublishedFailed:
		setStatus(ctx, e.db, task, StatusInit, "replay from published_failed")
		task.PublishResults = nil
		task.DraftHash = ""
		task.StepRetry = 0
		updateStepProgress(ctx, e.db, task, "", 0)
		return e.runStateMachine(ctx, task)

	default:
		return fmt.Errorf("task %s is not replayable (status=%s)", taskID, task.Status)
	}
}

func (e *Engine) GetTask(ctx context.Context, taskID string) (*WorkflowTask, error) {
	return loadTask(ctx, e.db, taskID)
}

// RecoverAll 启动时扫描所有非终态任务并恢复
func (e *Engine) RecoverAll(ctx context.Context) error {
	logger := logging.NewLogger("WorkflowEngineRecover")
	rows, err := e.db.QueryContext(ctx,
		`SELECT task_id, uid, skill_id, topic, novel_name, title, volume_name, chapter_number,
		        platform, status, session_id, draft_version, draft_hash,
		        md_path, trace_id, publish_results, accounts,
		        current_step, step_retry, step_updated_at, error_msg,
		        created_at, updated_at
		 FROM workflow_task
		 WHERE status IN ('init','fetch_draft','publishing','published','md_writing','md_written')
		 ORDER BY created_at ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var task WorkflowTask
		var resultsJSON string
		var accountsJSON string
		var stepUpdatedAt time.Time
		var createdAt time.Time
		var updatedAt time.Time
		var draftHash sql.NullString
		if err := rows.Scan(&task.TaskID, &task.UID, &task.SkillID, &task.Topic,
			&task.NovelName, &task.Title, &task.VolumeName, &task.ChapterNumber,
			&task.Platform, &task.Status, &task.SessionID, &task.DraftVersion, &draftHash,
			&task.MDPath, &task.TraceID, &resultsJSON, &accountsJSON,
			&task.CurrentStep, &task.StepRetry, &stepUpdatedAt, &task.ErrorMsg,
			&createdAt, &updatedAt); err != nil {
			logger.Error(logging.ErrDatabaseError, "recover scan error: err=%v", err)
			continue
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
		logger.Info("recovering task: task=%s status=%s session=%s", task.TaskID, task.Status, task.SessionID)
		e.runStateMachine(ctx, &task)
		count++
	}

	logger.Info("recovery complete: recovered %d pending tasks", count)
	logger.Close()
	return nil
}
