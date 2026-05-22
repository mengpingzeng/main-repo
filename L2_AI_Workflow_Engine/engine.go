package workflow_engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
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
	if err := validateInput(input); err != nil {
		return nil, err
	}

	task, isNew, err := e.upsertTask(ctx, input)
	if err != nil {
		return nil, err
	}

	if task.Status == StatusDone && !isNew {
		return task, nil
	}

	// Allow re-publish for done_partial and failed statuses
	if IsTerminal(task.Status) && !isNew && task.Status != StatusDonePartial && task.Status != StatusFailedGen && task.Status != StatusFailedMD {
		return task, nil
	}

	// Reset for re-publish
	if task.Status == StatusDonePartial || task.Status == StatusFailedGen || task.Status == StatusFailedMD {
		task.Status = StatusPublishing
		task.PublishResults = nil
		task.CurrentStep = ""
		task.StepRetry = 0
	}

	if err := e.runStateMachine(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (e *Engine) upsertTask(ctx context.Context, input PublishInput) (*WorkflowTask, bool, error) {
	existing, err := loadTask(ctx, e.db, taskIDFromInput(input))
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		// 刷新前端可能更新的字段
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
		if updated {
			accountsJSON, _ := json.Marshal(existing.Accounts)
			e.db.ExecContext(ctx,
				`UPDATE workflow_task SET novel_name=?, title=?, volume_name=?, chapter_number=?, accounts=?, updated_at=NOW() WHERE task_id=?`,
				existing.NovelName, existing.Title, existing.VolumeName, existing.ChapterNumber, string(accountsJSON), existing.TaskID)
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
	var draftContent string
	for !IsTerminal(task.Status) {
		log.Printf("[task=%s] step start: status=%s", task.TaskID, task.Status)
		e.pushWS(task, task.Status, "running", "")

		var next string
		var err error

		switch task.Status {
		case StatusInit, StatusFetchDraft:
			next, draftContent, err = stepFetchDraft(ctx, task, e.draftFetcher, e.db)

		case StatusPublishing:
			if draftContent == "" {
				draftContent, err = e.draftFetcher.Fetch(ctx, task.TaskID, task.SessionID, task.DraftVersion)
				if err != nil {
					log.Printf("[task=%s] step publish fetch draft error: %v", task.TaskID, err)
					return err
				}
			}
			next, err = stepPublishing(ctx, task, e.c1, e.db, draftContent)

		case StatusPublished:
			next = StatusMDWriting
			setStatus(ctx, e.db, task, next, "")

		case StatusMDWriting:
			if draftContent == "" {
				draftContent, err = e.draftFetcher.Fetch(ctx, task.TaskID, task.SessionID, task.DraftVersion)
				if err != nil {
					log.Printf("[task=%s] step md write fetch draft error: %v", task.TaskID, err)
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

		case StatusMDWritten:
			next = resolveFinalStatus(task)
			setStatus(ctx, e.db, task, next, "")
			e.pushWS(task, next, "success", "完成")

		default:
			log.Printf("[task=%s] unknown status: %s", task.TaskID, task.Status)
			return fmt.Errorf("unknown status: %s", task.Status)
		}

		if err != nil {
			log.Printf("[task=%s] step error: status=%s, next=%s, err=%v", task.TaskID, task.Status, next, err)
			return err
		}
		log.Printf("[task=%s] step done: status=%s -> next=%s", task.TaskID, task.Status, next)
		task.Status = next
	}
	log.Printf("[task=%s] state machine finished: status=%s", task.TaskID, task.Status)
	return nil
}

// Replay 重放失败或部分成功的任务
func (e *Engine) Replay(ctx context.Context, taskID string) error {
	task, err := loadTask(ctx, e.db, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task %s not found", taskID)
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

	default:
		return fmt.Errorf("task %s is not replayable (status=%s)", taskID, task.Status)
	}
}

// RecoverAll 启动时扫描所有非终态任务并恢复
func (e *Engine) RecoverAll(ctx context.Context) error {
	rows, err := e.db.QueryContext(ctx,
		`SELECT task_id, uid, skill_id, topic, novel_name, title, volume_name, chapter_number,
		        platform, status, session_id, draft_version,
		        md_path, trace_id, publish_results, accounts,
		        current_step, step_retry, step_updated_at, error_msg
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
		if err := rows.Scan(&task.TaskID, &task.UID, &task.SkillID, &task.Topic,
			&task.NovelName, &task.Title, &task.VolumeName, &task.ChapterNumber,
			&task.Platform, &task.Status, &task.SessionID, &task.DraftVersion,
			&task.MDPath, &task.TraceID, &resultsJSON, &accountsJSON,
			&task.CurrentStep, &task.StepRetry, &stepUpdatedAt, &task.ErrorMsg); err != nil {
			log.Printf("recover scan error: %v", err)
			continue
		}
		task.StepUpdatedAt = stepUpdatedAt
		if resultsJSON != "" {
			json.Unmarshal([]byte(resultsJSON), &task.PublishResults)
		}
		if accountsJSON != "" {
			json.Unmarshal([]byte(accountsJSON), &task.Accounts)
		}
		log.Printf("recovering task %s (status=%s)", task.TaskID, task.Status)
		e.runStateMachine(ctx, &task)
		count++
	}

	log.Printf("recovered %d pending tasks", count)
	return nil
}
