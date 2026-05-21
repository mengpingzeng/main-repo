package a4md

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// readWithFallback 从 OSS 读取内容，失败时返回 fallback 字符串
func (s *Service) readWithFallback(ctx context.Context, ossPath string, fallback string) string {
	reader, err := s.storage.Get(ctx, ossPath)
	if err != nil {
		s.logger.Warn("a4md: OSS read degraded", "path", ossPath, "error", err)
		return fallback
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		s.logger.Warn("a4md: OSS read content error", "path", ossPath, "error", err)
		return fallback
	}
	return string(content)
}

// readExistingMD 从 OSS 读取现有 MD
// 若文件不存在 → 返回 ErrWriteMDNotExist
func (s *Service) readExistingMD(ctx context.Context, taskID string, draftVersion int) (string, error) {
	ossPath := buildOSSPath(taskID, draftVersion, 0)
	reader, err := s.storage.Get(ctx, ossPath)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %w", ErrWriteMDNotExist, ossPath, err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("a4md: read existing MD content failed: %w", err)
	}
	return string(content), nil
}

// readDraftContent 读 draft 原文，失败降级为路径占位
func (s *Service) readDraftContent(ctx context.Context, sessionID string, draftVersion int) string {
	ossPath := fmt.Sprintf("sessions/%s/draft_v%d.md", sessionID, draftVersion)
	fallback := fmt.Sprintf("*(draft 原文无法读取: %s)*", ossPath)
	return s.readWithFallback(ctx, ossPath, fallback)
}

// readEpisodeSummary 读 episode 摘要原文，失败降级为提示文本
func (s *Service) readEpisodeSummary(ctx context.Context, episodeID string) (userIntent, decisions string) {
	sessionID, epochNo, err := parseEpisodeID(episodeID)
	if err != nil {
		s.logger.Warn("a4md: invalid episode ID", "episodeID", episodeID, "err", err)
		return "*(摘要读取失败: ID格式错误)*", "*(摘要读取失败: ID格式错误)*"
	}

	ossPath := fmt.Sprintf("sessions/%s/ep%d.md", sessionID, epochNo)
	summary := s.readWithFallback(ctx, ossPath, "")

	if summary == "" {
		return "*(摘要读取失败)*", "*(摘要读取失败)*"
	}
	return parseEpisodeSummary(summary)
}

// parseEpisodeID 从 episodeID 中解析 sessionID 和 epochNo
// 约定格式：ep_{session_id}_{epoch_no}
func parseEpisodeID(episodeID string) (sessionID string, epochNo int, err error) {
	idx := strings.LastIndex(episodeID, "_")
	if idx == -1 || idx < 3 {
		return "", 0, fmt.Errorf("a4md: invalid episode ID format: %s", episodeID)
	}
	sessionID = episodeID[3:idx]
	epochNo, err = strconv.Atoi(episodeID[idx+1:])
	if err != nil {
		return "", 0, fmt.Errorf("a4md: failed to parse epoch from episode ID %s: %w", episodeID, err)
	}
	return sessionID, epochNo, nil
}

// parseEpisodeSummary 从 episode 摘要文件中解析 user_intent 和 decisions
// 摘要文件约定格式：
//
//	## 用户核心诉求
//	{user_intent}
//	## Agent 关键决定
//	{decisions}
func parseEpisodeSummary(raw string) (userIntent, decisions string) {
	parts := strings.SplitN(raw, "## Agent 关键决定", 2)
	userIntentPart := ""
	decisionsPart := ""
	if len(parts) >= 1 {
		userIntentPart = strings.TrimPrefix(parts[0], "## 用户核心诉求")
		userIntentPart = strings.TrimSpace(userIntentPart)
	}
	if len(parts) >= 2 {
		decisionsPart = strings.TrimSpace(parts[1])
	}
	if userIntentPart == "" {
		userIntentPart = "*(摘要解析失败)*"
	}
	if decisionsPart == "" {
		decisionsPart = "*(摘要解析失败)*"
	}
	return userIntentPart, decisionsPart
}

// readPlatformStats 从 MySQL 读取指定 task 的统计数据，失败降级返回 nil
func (s *Service) readPlatformStats(ctx context.Context, taskID string) []StatItem {
	if s.statsRepo == nil {
		return nil
	}
	items, err := s.statsRepo.GetStatsByTaskID(ctx, taskID)
	if err != nil {
		s.logger.Warn("a4md: MySQL platform_stats read degraded", "task_id", taskID, "error", err)
		return nil
	}
	return items
}

// readPlatformStatsByPeriod 按时间段读取统计数据，失败降级返回 nil
func (s *Service) readPlatformStatsByPeriod(ctx context.Context, taskID string, period string) []StatItem {
	if s.statsRepo == nil {
		return nil
	}
	items, err := s.statsRepo.GetStatsByPeriod(ctx, taskID, period)
	if err != nil {
		s.logger.Warn("a4md: MySQL platform_stats read degraded", "task_id", taskID, "period", period, "error", err)
		return nil
	}
	return items
}

// isSectionExists 检查 MD 中是否已包含指定 section
func isSectionExists(mdContent, period string) bool {
	anchor := fmt.Sprintf("### %s数据", period)
	return strings.Contains(mdContent, anchor)
}

// buildTemplateData 组装模板数据，对所有动态字段执行 MD 转义
func (s *Service) buildTemplateData(ctx context.Context, input WriteMDInput) *TemplateData {
	data := &TemplateData{
		UID:          input.UID,
		TaskID:       input.TaskID,
		Topic:        escapeForMD(input.Topic),
		CreatedAt:    input.CreatedAt.UTC().Format(time.RFC3339),
		SkillID:      input.SkillID,
		SkillName:    input.SkillName,
		SkillVersion: input.SkillVersion,
		Model:        input.Model,
		DraftVersion: input.DraftVersion,
	}

	for _, sess := range input.Sessions {
		ts := TemplateSession{
			SessionID:    sess.SessionID,
			StartedAt:    sess.StartedAt.UTC().Format(time.RFC3339),
			EndedAt:      sess.EndedAt.UTC().Format(time.RFC3339),
			MessageCount: sess.MessageCount,
			DraftVersion: sess.DraftVersion,
		}
		for _, ep := range sess.Episodes {
			ts.Episodes = append(ts.Episodes, TemplateEpisode{
				EpisodeID:  ep.EpisodeID,
				EpochNo:    ep.EpochNo,
				UserIntent: escapeForMD(ep.UserIntent),
				Decisions:  escapeForMD(ep.Decisions),
			})
		}
		data.Sessions = append(data.Sessions, ts)
	}

	data.Products = TemplateProducts{
		XhsText:      escapeForMD(input.Products.XhsText),
		WechatHTML:   escapeForMD(input.Products.WechatHTML),
		FanqieNovel:  escapeForMD(input.Products.FanqieNovel),
		ZhulangNovel: escapeForMD(input.Products.ZhulangNovel),
		Others:       escapeProductMap(input.Products.Others),
	}

	for _, pr := range input.PublishResults {
		data.PublishResults = append(data.PublishResults, TemplatePublishResult{
			AccountID:     pr.AccountID,
			Platform:      pr.Platform,
			Status:        pr.Status,
			PostID:        escapeForMD(pr.PostID),
			PostURL:       escapeForMD(pr.PostURL),
			ErrorCode:     pr.ErrorCode,     // 系统错误码，不转义（短码无 MD 语义）
			MaskedDisplay: pr.MaskedDisplay, // 脱敏展示名，不转义（不含 MD 特殊字符）
		})
	}

	return data
}
