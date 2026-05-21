package a4md

import (
	"context"
)

// Service A4 MD 沉淀的核心接口
// 仅被 Workflow Engine 调用
type Service struct {
	storage   Storage
	statsRepo StatsRepository
	engine    *TemplateEngine
	taskLocks *taskLockSet
	logger    Logger
}

// Logger 日志接口，调用方注入
type Logger interface {
	Warn(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// NewService 创建 A4 MD 服务实例
func NewService(storage Storage, statsRepo StatsRepository, engine *TemplateEngine, logger Logger) *Service {
	if logger == nil {
		logger = &noopLogger{}
	}
	return &Service{
		storage:   storage,
		statsRepo: statsRepo,
		engine:    engine,
		taskLocks: newTaskLockSet(),
		logger:    logger,
	}
}

// WriteMD 首次生成任务 MD 并上传 OSS
// 返回 OSS 文件路径，格式：tasks/{task_id}/archive/draft_v{draft_version}.md
func (s *Service) WriteMD(ctx context.Context, input WriteMDInput) (WriteMDResult, error) {
	data := s.buildTemplateData(ctx, input)
	mdContent, err := s.engine.Render(data)
	if err != nil {
		return WriteMDResult{}, wrapErr(ErrTemplateRenderFail, err)
	}

	// 检查大小，超过 5MB 走分片
	if len(mdContent) > maxFileSize {
		mainPath, err := s.writeLargeContent(ctx, input.TaskID, input.DraftVersion, []byte(mdContent))
		if err != nil {
			return WriteMDResult{}, err
		}
		return WriteMDResult{MDPath: mainPath}, nil
	}

	ossPath := buildOSSPath(input.TaskID, input.DraftVersion, 0)
	if err := s.writeWithRetry(ctx, ossPath, []byte(mdContent)); err != nil {
		return WriteMDResult{}, err
	}

	return WriteMDResult{MDPath: ossPath}, nil
}

// AppendStats 追加写数据反馈段落（24h/7d/30d）
// 同一 task 必须串行调用，内部保证幂等（同 section 不重复）
func (s *Service) AppendStats(ctx context.Context, input AppendStatsInput) error {
	unlock := s.taskLocks.acquire(input.TaskID)
	defer unlock()

	existingMD, err := s.readExistingMD(ctx, input.TaskID, input.DraftVersion)
	if err != nil {
		return fmtAppendErr("AppendStats requires existing MD", err)
	}

	if isSectionExists(existingMD, input.StatsPeriod) {
		return nil
	}

	stats := input.Stats
	if len(stats) == 0 && s.statsRepo != nil {
		stats = s.readPlatformStatsByPeriod(ctx, input.TaskID, input.StatsPeriod)
	}

	appendSection, err := s.engine.RenderStatsSection(input.StatsPeriod, stats)
	if err != nil {
		return wrapErr(ErrTemplateRenderFail, err)
	}

	fullMD := existingMD + "\n\n" + appendSection

	if len(fullMD) > maxFileSize {
		return s.splitAndWrite(ctx, input.TaskID, input.DraftVersion, []byte(fullMD))
	}

	ossPath := buildOSSPath(input.TaskID, input.DraftVersion, 0)
	return s.writeWithRetry(ctx, ossPath, []byte(fullMD))
}
