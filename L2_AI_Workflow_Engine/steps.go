package workflow_engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"clawstudios/pkg/logging"
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
	l := logging.FromContext(ctx)

	if task.CurrentStep == StatusPublishing && len(task.PublishResults) > 0 {
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

	product := ProductContent{
		Text:          stripMarkdown(draftContent),
		NovelName:     task.NovelName,
		Title:         task.Title,
		VolumeName:    task.VolumeName,
		ChapterNumber: task.ChapterNumber,
	}

	results, err := c1.Publish(ctx, map[string]ProductContent{
		task.Platform: product,
	}, task.Accounts)
	if err != nil {
		if l != nil {
			l.Error(logging.ErrWorkflowError, "发布调用失败: task=%s platform=%s err=%v", task.TaskID, task.Platform, err)
		}
		return StatusPublishing, err
	}

	for _, r := range results {
		if l != nil {
			if r.Status == "ok" {
				l.Info("账号发布成功: account=%s platform=%s postID=%s session=%s ch=%d",
					r.AccountID, r.Platform, r.PostID, task.SessionID, task.ChapterNumber)
			} else {
				l.Warn(logging.ErrWorkflowError, "账号发布失败: account=%s platform=%s status=%s errorCode=%s display=%s",
					r.AccountID, r.Platform, r.Status, r.ErrorCode, r.MaskedDisplay)
			}
		}
	}

	allFailed := true
	for _, r := range results {
		if r.Status == "ok" {
			allFailed = false
			break
		}
	}
	if allFailed {
		setStatus(ctx, db, task, StatusPublishedFailed, "all accounts failed to publish")
		return StatusPublishedFailed, nil
	}

	task.PublishResults = results
	setPublishResults(ctx, db, task, StatusPublished)
	return StatusPublished, nil
}

func stepMDWriting(ctx context.Context, task *WorkflowTask, a4 MDWriter, db *sql.DB, draftContent string) (string, error) {
	l := logging.FromContext(ctx)

	if task.MDPath != "" {
		return StatusMDWritten, nil
	}

	if task.StepRetry >= MaxRetriesMDWriting {
		if l != nil {
			l.Error(logging.ErrWorkflowError, "文档沉淀重试耗尽: task=%s retries=%d", task.TaskID, task.StepRetry)
		}
		setStatus(ctx, db, task, StatusFailedMD, "a4 write md failed")
		return StatusFailedMD, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	updateStepProgress(ctx, db, task, StatusMDWriting, task.StepRetry+1)

	var publishedCount int
	if task.NovelName != "" {
		db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM workflow_task WHERE novel_name = ? AND status IN ('done', 'done_partial')`,
			task.NovelName).Scan(&publishedCount)
	}
	publishedCount++

	mdPath, err := a4.WriteMD(ctx, MDWriteRequest{
		TaskID:                task.TaskID,
		UID:                   task.UID,
		SkillID:               task.SkillID,
		SkillName:             task.SkillName,
		Model:                 task.Model,
		Topic:                 task.Topic,
		NovelName:             task.NovelName,
		VolumeName:            task.VolumeName,
		Title:                 task.Title,
		ChapterNumber:         task.ChapterNumber,
		PublishedChapterCount: publishedCount,
		SessionID:             task.SessionID,
		DraftVersion:          task.DraftVersion,
		PublishResults:        task.PublishResults,
		TraceID:               task.TraceID,
	})
	if err != nil {
		if l != nil {
			l.Error(logging.ErrWorkflowError, "文档沉淀写入失败: task=%s retry=%d err=%v", task.TaskID, task.StepRetry, err)
		}
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
	text = stripMetaCommentary(text)
	text = extractNovelContent(text)
	text = normalizeNewlines(text)
	re := importRegexp()
	text = re.heading.ReplaceAllString(text, "$1")
	text = re.chapterHeader.ReplaceAllString(text, "")
	text = re.bold.ReplaceAllString(text, "$1")
	text = re.italic.ReplaceAllString(text, "$1")
	text = strings.TrimSpace(text)
	return text
}

var metaCommentaryRx = regexp.MustCompile(`(?m)^.*已(直接)?输出.*current_draft\.md.*$\n?`)
var excessNewlineRx = regexp.MustCompile(`\n{3,}`)

func stripMetaCommentary(text string) string {
	return metaCommentaryRx.ReplaceAllString(text, "")
}

func normalizeNewlines(text string) string {
	return excessNewlineRx.ReplaceAllString(text, "\n\n")
}

func extractNovelContent(draftContent string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(draftContent), &data); err != nil {
		return draftContent
	}
	if content, ok := data["content"]; ok {
		if s, ok := content.(string); ok {
			s = strings.ReplaceAll(s, "\\n\\n", "\n\n")
			s = strings.ReplaceAll(s, "\\n", "\n")
			return s
		}
	}
	return draftContent
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
