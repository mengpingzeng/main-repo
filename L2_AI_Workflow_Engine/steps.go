package workflow_engine

import (
	"context"
	"database/sql"
	"regexp"
	"time"
)

func stepFetchDraft(ctx context.Context, task *WorkflowTask, fetcher DraftFetcher, db *sql.DB) (nextStatus string, draftContent string, err error) {
	if task.StepRetry >= MaxRetriesFetchDraft {
		setStatus(ctx, db, task, StatusFailedGen, "oss draft fetch exhausted 3 retries")
		return StatusFailedGen, "", nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	updateStepProgress(ctx, db, task, StatusFetchDraft, task.StepRetry+1)

	draft, err := fetcher.Fetch(ctx, task.TaskID, task.SessionID, task.DraftVersion)
	if err != nil {
		return task.Status, "", nil
	}

	setStepResult(ctx, db, task, StatusPublishing, "fetch_draft")
	return StatusPublishing, draft, nil
}

func stepPublishing(ctx context.Context, task *WorkflowTask, c1 C1Publisher, db *sql.DB, draftContent string) (string, error) {
	if task.CurrentStep == StatusPublishing && len(task.PublishResults) > 0 {
		// Only skip if at least one publish succeeded; if all failed, retry
		allFailed := true
		for _, r := range task.PublishResults {
			if r.Status == "ok" {
				allFailed = false
				break
			}
		}
		if !allFailed {
			return StatusPublished, nil
		}
		// All failed: clear results and retry
		task.PublishResults = nil
		task.CurrentStep = ""
	}

	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	setStatus(ctx, db, task, StatusPublishing, "")

	if realAdapter, ok := c1.(*RealC1Adapter); ok {
		realAdapter.SetNovelMeta(task.NovelName, task.Title)
		realAdapter.TaskID = task.TaskID
		realAdapter.SessionID = task.SessionID
		realAdapter.TraceID = task.TraceID
	}

	results, err := c1.Publish(ctx, map[string]ProductContent{
		task.Platform: {
			Text:          stripMarkdown(draftContent),
			NovelName:     task.NovelName,
			Title:         task.Title,
			VolumeName:    task.VolumeName,
			ChapterNumber: task.ChapterNumber,
		},
	}, task.Accounts)
	if err != nil {
		return StatusPublishing, err
	}

	task.PublishResults = results
	setPublishResults(ctx, db, task, StatusPublished)
	return StatusPublished, nil
}

func stepMDWriting(ctx context.Context, task *WorkflowTask, a4 MDWriter, db *sql.DB, draftContent string) (string, error) {
	if task.MDPath != "" {
		return StatusMDWritten, nil
	}

	if task.StepRetry >= MaxRetriesMDWriting {
		setStatus(ctx, db, task, StatusFailedMD, "a4 write md failed")
		return StatusFailedMD, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	updateStepProgress(ctx, db, task, StatusMDWriting, task.StepRetry+1)

	mdPath, err := a4.WriteMD(ctx, MDWriteRequest{
		TaskID:         task.TaskID,
		UID:            task.UID,
		SkillID:        task.SkillID,
		Topic:          task.Topic,
		Platform:       task.Platform,
		SessionID:      task.SessionID,
		DraftVersion:   task.DraftVersion,
		PublishResults: task.PublishResults,
		TraceID:        task.TraceID,
	})
	if err != nil {
		return task.Status, nil
	}

	task.MDPath = mdPath
	db.ExecContext(ctx,
		`UPDATE workflow_task SET md_path=?, updated_at=? WHERE task_id=?`,
		mdPath, time.Now().UTC(), task.TaskID)
	setStepResult(ctx, db, task, StatusMDWritten, "md_writing")
	return StatusMDWritten, nil
}

func resolveFinalStatus(task *WorkflowTask) string {
	for _, r := range task.PublishResults {
		if r.Status != "ok" {
			return StatusDonePartial
		}
	}
	return StatusDone
}

func stripMarkdown(text string) string {
	re := importRegexp()
	text = re.heading.ReplaceAllString(text, "$1")
	text = re.chapterHeader.ReplaceAllString(text, "")
	text = re.bold.ReplaceAllString(text, "$1")
	text = re.italic.ReplaceAllString(text, "$1")
	return text
}

var mdRegexp *markdownRegexp

type markdownRegexp struct {
	heading       *regexp.Regexp
	bold          *regexp.Regexp
	italic        *regexp.Regexp
	chapterHeader *regexp.Regexp
}

func importRegexp() *markdownRegexp {
	if mdRegexp == nil {
		mdRegexp = &markdownRegexp{
			heading:       regexp.MustCompile(`(?m)^#{1,6}\s+(.*)`),
			bold:          regexp.MustCompile(`\*\*(.+?)\*\*`),
			italic:        regexp.MustCompile(`\*(.+?)\*`),
			chapterHeader: regexp.MustCompile(`(?m)^([Cc][Hh]\d+[:\s].*|第[零一二三四五六七八九十百千\d]+[章节回卷]\s*.*|Chapter\s+\d+.*|Volume\s+\d+.*)\n*`),
		}
	}
	return mdRegexp
}
