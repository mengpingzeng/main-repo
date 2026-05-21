package workflow_engine

import (
	"context"

	c1 "clawstudios/l1_ai_releaser/services/c1_publisher"
)

type RealC1Adapter struct {
	publisher    *c1.RealPublisher
	TaskID       string
	SessionID    string
	TraceID      string
	novelName    string
	chapterTitle string
}

func NewRealC1Adapter(publisher *c1.RealPublisher, taskID, sessionID, traceID string) *RealC1Adapter {
	return &RealC1Adapter{
		publisher: publisher,
		TaskID:    taskID,
		SessionID: sessionID,
		TraceID:   traceID,
	}
}

func (a *RealC1Adapter) SetNovelMeta(novelName, chapterTitle string) {
	a.novelName = novelName
	a.chapterTitle = chapterTitle
}

func (a *RealC1Adapter) Publish(ctx context.Context, products map[string]ProductContent, accounts []AccountRef) ([]PublishResult, error) {
	c1p := make(map[string]c1.ProductContent)
	for k, v := range products {
		pc := c1.ProductContent{
			Text:          v.Text,
			HTML:          v.HTML,
			Tags:          v.Tags,
			NovelName:     v.NovelName,
			Title:         v.Title,
			VolumeName:    v.VolumeName,
			ChapterNumber: v.ChapterNumber,
		}
		if pc.NovelName == "" && a.novelName != "" {
			pc.NovelName = a.novelName
		}
		if pc.Title == "" && a.chapterTitle != "" {
			pc.Title = a.chapterTitle
		}
		c1p[k] = pc
	}

	c1a := make([]c1.AccountRef, len(accounts))
	for i, acc := range accounts {
		c1a[i] = c1.AccountRef{
			AccountID: acc.AccountID,
			UID:       acc.UID,
			Platform:  acc.Platform,
		}
	}

	resp, err := a.publisher.Publish(ctx, c1.PublishRequest{
		TaskID:    a.TaskID,
		Products:  c1p,
		Accounts:  c1a,
		TraceID:   a.TraceID,
		SessionID: a.SessionID,
	})
	if err != nil {
		return nil, err
	}

	out := make([]PublishResult, len(resp.Results))
	for i, r := range resp.Results {
		out[i] = PublishResult{
			AccountID:     r.AccountID,
			Platform:      r.Platform,
			Status:        r.Status,
			PostID:        r.PostID,
			ErrorCode:     r.ErrorCode,
			MaskedDisplay: r.MaskedDisplay,
		}
	}
	return out, nil
}

func (a *RealC1Adapter) Close() error {
	if a.publisher != nil {
		return a.publisher.Close()
	}
	return nil
}

var _ C1Publisher = (*RealC1Adapter)(nil)
