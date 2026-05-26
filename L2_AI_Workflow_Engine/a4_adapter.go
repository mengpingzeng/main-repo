package workflow_engine

import (
	"context"
	"encoding/json"
	"time"

	"a4md"
	"clawstudios/pkg/logging"
)

type RealMDWriterAdapter struct {
	service *a4md.Service
}

func NewRealMDWriterAdapter(storageDir string) (*RealMDWriterAdapter, error) {
	storage, err := a4md.NewLocalFSStorage(storageDir)
	if err != nil {
		return nil, err
	}

	engine, err := a4md.NewTemplateEngine("v1")
	if err != nil {
		return nil, err
	}

	svc := a4md.NewService(storage, nil, engine, nil)
	return &RealMDWriterAdapter{service: svc}, nil
}

func (a *RealMDWriterAdapter) WriteMD(ctx context.Context, req MDWriteRequest) (string, error) {
	l := logging.FromContext(ctx)

	if l != nil {
		reqJSON, _ := json.Marshal(req)
		l.Info("文档沉淀入参: task=%s novel=%s title=%s ch=%d skill=%s model=%s",
			req.TaskID, req.NovelName, req.Title, req.ChapterNumber, req.SkillID, req.Model)
		l.Info("文档沉淀详细信息: %s", string(reqJSON))
	}

	publishResults := make([]a4md.PublishResult, len(req.PublishResults))
	for i, r := range req.PublishResults {
		publishResults[i] = a4md.PublishResult{
			AccountID:     r.AccountID,
			Platform:      r.Platform,
			Status:        r.Status,
			PostID:        r.PostID,
			ErrorCode:     r.ErrorCode,
			MaskedDisplay: r.MaskedDisplay,
		}
	}

	sessions := make([]a4md.SessionInfo, len(req.Sessions))
	for i, s := range req.Sessions {
		episodes := make([]a4md.EpisodeSummary, len(s.Episodes))
		for j, e := range s.Episodes {
			episodes[j] = a4md.EpisodeSummary{
				EpisodeID:  e.EpisodeID,
				EpochNo:    e.EpochNo,
				UserIntent: e.UserIntent,
				Decisions:  e.Decisions,
				SummaryOSS: e.SummaryOSS,
			}
		}
		sessions[i] = a4md.SessionInfo{
			SessionID:    s.SessionID,
			StartedAt:    s.StartedAt,
			EndedAt:      s.EndedAt,
			MessageCount: s.MessageCount,
			Episodes:     episodes,
			DraftVersion: s.DraftVersion,
		}
	}

	products := a4md.Products{
		XhsText:      req.Products.XhsText,
		WechatHTML:   req.Products.WechatHTML,
		FanqieNovel:  req.Products.FanqieNovel,
		ZhulangNovel: req.Products.ZhulangNovel,
		Others:       req.Products.Others,
	}

	createdAt := req.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	result, err := a.service.WriteMD(ctx, a4md.WriteMDInput{
		TaskID:                req.TaskID,
		UID:                   req.UID,
		Topic:                 req.Topic,
		NovelName:             req.NovelName,
		VolumeName:            req.VolumeName,
		Title:                 req.Title,
		ChapterNumber:         req.ChapterNumber,
		PublishedChapterCount: req.PublishedChapterCount,
		CreatedAt:             createdAt,
		SkillID:               req.SkillID,
		SkillName:             req.SkillName,
		Model:                 req.Model,
		Sessions:              sessions,
		DraftVersion:          req.DraftVersion,
		Products:              products,
		PublishResults:        publishResults,
		EpisodeIDs:            req.EpisodeIDs,
		TraceID:               req.TraceID,
	})
	if err != nil {
		if l != nil {
			l.Error(logging.ErrWorkflowError, "文档沉淀写入失败: task=%s err=%v", req.TaskID, err)
		}
		return "", err
	}

	if l != nil {
		l.Info("文档沉淀出参: task=%s mdPath=%s", req.TaskID, result.MDPath)
	}
	return result.MDPath, nil
}

var _ MDWriter = (*RealMDWriterAdapter)(nil)
