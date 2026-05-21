// Package c1_publisher 提供 RealPublisher 实现。
package c1_publisher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

// RealPublisher C1 发布器的真实实现。
// 通过 HTTP 调用 A1 凭证服务 + 平台 Adapters + 持久化存储。
type RealPublisher struct {
	adapters map[string]PublishAdapter // key=platform
	store    Store
	config   Config
}

// NewRealPublisher 创建 RealPublisher。
// cfg.DB != nil 时使用 MySQL 持久化存储，否则使用内存存储。
func NewRealPublisher(cfg Config) *RealPublisher {
	var s Store
	if cfg.DB != nil {
		s = NewMySQLPublishStore(cfg.DB)
	} else {
		s = NewPublishStore()
	}
	rp := &RealPublisher{
		adapters: make(map[string]PublishAdapter),
		store:    s,
		config:   cfg,
	}
	for _, a := range cfg.Adapters {
		rp.adapters[a.Platform()] = a
	}
	return rp
}

// Publish 发布主流程。
// Step 1: 获取凭证（按 account_id 去重，失败用 credFetchResult 标记）
// Step 2: 构建发布单元列表
// Step 3: goroutine 并发发布（部分成功语义）
// Step 4: 逐条 UPSERT publish_record
// Step 5: 汇总返回
func (rp *RealPublisher) Publish(ctx context.Context, req PublishRequest) (*PublishResponse, error) {
	if err := rp.validateRequest(req); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	credCache := make(map[string]*credFetchResult)
	for _, acc := range req.Accounts {
		if _, exists := credCache[acc.AccountID]; exists {
			continue
		}
		credResp, err := FetchCredential(ctx, rp.config.A1BaseURL, acc.AccountID, acc.UID)
		if err != nil {
			credCache[acc.AccountID] = &credFetchResult{err: err}
			continue
		}
		credCache[acc.AccountID] = &credFetchResult{resp: credResp}
	}

	units := rp.buildPublishUnits(req, credCache)
	results := rp.concurrentPublish(ctx, units)

	for _, result := range results {
		if err := rp.store.Upsert(ctx, NewPublishRecord(req.TaskID, result, req.SkillID, req.SessionID, result.NovelName)); err != nil {
			logError("publish_record write failed",
				"task_id", req.TaskID,
				"account_id", result.AccountID,
				"error", err,
			)
		}
	}

	summary := summarize(results)
	return &PublishResponse{
		TaskID:  req.TaskID,
		Results: results,
		Summary: summary,
	}, nil
}

func (rp *RealPublisher) Health(_ context.Context) error {
	return nil
}

func (rp *RealPublisher) Close() error {
	return nil
}

// GetStore 返回发布记录存储（供测试和 demo 用）。
func (rp *RealPublisher) GetStore() Store {
	return rp.store
}

// --- 内部方法 ---

func (rp *RealPublisher) validateRequest(req PublishRequest) error {
	if req.TaskID == "" {
		return fmt.Errorf("task_id is required")
	}
	if len(req.Accounts) == 0 {
		return fmt.Errorf("accounts list is empty")
	}
	if len(req.Products) == 0 {
		return fmt.Errorf("products map is empty")
	}
	for i, acc := range req.Accounts {
		if acc.AccountID == "" {
			return fmt.Errorf("accounts[%d].account_id is required", i)
		}
		if acc.UID == "" {
			return fmt.Errorf("accounts[%d].uid is required", i)
		}
		if acc.Platform == "" {
			return fmt.Errorf("accounts[%d].platform is required", i)
		}
	}
	return nil
}

func (rp *RealPublisher) buildPublishUnits(req PublishRequest, credCache map[string]*credFetchResult) []*PublishUnit {
	var units []*PublishUnit

	for _, acc := range req.Accounts {
		product, productOK := req.Products[acc.Platform]
		if !productOK {
			units = append(units, rp.makeFailUnit(req.TaskID, acc, product,
				ErrCodeMissingProduct, "no product content for platform "+acc.Platform))
			continue
		}

		adapter, adapterOK := rp.adapters[acc.Platform]
		if !adapterOK {
			units = append(units, rp.makeFailUnit(req.TaskID, acc, product,
				ErrCodeUnsupportedPlatform, "no adapter for platform "+acc.Platform))
			continue
		}

		if checkErr := adapter.CheckInput(product); checkErr != "" {
			units = append(units, rp.makeFailUnit(req.TaskID, acc, product,
				ErrCodeInputInvalid, checkErr))
			continue
		}

		cr := credCache[acc.AccountID]
		if cr == nil || cr.err != nil {
			errCode := ErrCodeCredentialFailed
			errMsg := "failed to get credentials from A1"
			if cr != nil && cr.err != nil {
				failResult := handleCredentialError(acc.AccountID, cr.err)
				errCode = failResult.ErrorCode
				errMsg = failResult.ErrorMessage
			}
			units = append(units, rp.makeFailUnit(req.TaskID, acc, product, errCode, errMsg))
			continue
		}

		contentHash := hashContent(product)
		units = append(units, &PublishUnit{
			TaskID:      req.TaskID,
			AccountID:   acc.AccountID,
			UID:         acc.UID,
			Platform:    acc.Platform,
			Product:     product,
			Credentials: cr.resp.Credentials,
			ContentHash: contentHash,
			SkillID:     req.SkillID,
			SessionID:   req.SessionID,
			MaskedDisplay: cr.resp.MaskedDisplay,
		})
	}

	return units
}

func (rp *RealPublisher) makeFailUnit(taskID string, acc AccountRef, product ProductContent, code, msg string) *PublishUnit {
	return &PublishUnit{
		TaskID:    taskID,
		AccountID: acc.AccountID,
		UID:       acc.UID,
		Platform:  acc.Platform,
		Product:   product,
		ErrorCode: code,
		ErrorMsg:  msg,
	}
}

func hashContent(p ProductContent) string {
	data := p.Text + p.HTML
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

func summarize(results []PublishResult) PublishSummary {
	s := PublishSummary{Total: len(results)}
	for _, r := range results {
		if r.Status == "ok" {
			s.Succeeded++
		} else {
			s.Failed++
		}
	}
	return s
}

// --- 并发发布 ---

func (rp *RealPublisher) concurrentPublish(ctx context.Context, units []*PublishUnit) []PublishResult {
	n := len(units)
	if n == 0 {
		return nil
	}

	results := make([]PublishResult, n)
	var mu sync.Mutex

	g, ctx := withContext(ctx)

	if rp.config.ConcurrencyLimit > 0 && rp.config.ConcurrencyLimit < n {
		sem := make(chan struct{}, rp.config.ConcurrencyLimit)
		for i, u := range units {
			i, u := i, u
			g.goWait(func() error {
				sem <- struct{}{}
				defer func() { <-sem }()
				return rp.publishOneUnit(ctx, u, &results, i, &mu)
			})
		}
	} else {
		for i, u := range units {
			i, u := i, u
			g.goWait(func() error {
				return rp.publishOneUnit(ctx, u, &results, i, &mu)
			})
		}
	}

	_ = g.wait()
	return results
}

func (rp *RealPublisher) publishOneUnit(
	ctx context.Context,
	unit *PublishUnit,
	results *[]PublishResult,
	index int,
	mu *sync.Mutex,
) error {
	defer func() {
		if r := recover(); r != nil {
			logError("publish goroutine panic",
				"task_id", unit.TaskID,
				"account_id", unit.AccountID,
				"platform", unit.Platform,
				"panic", fmt.Sprintf("%v", r),
			)
			mu.Lock()
			(*results)[index] = PublishResult{
				AccountID:     unit.AccountID,
				UID:           unit.UID,
				Platform:      unit.Platform,
				Status:        "fail",
				ErrorCode:     ErrCodeInternalPanic,
				ErrorMessage:  "publisher internal panic",
				NovelName:     unit.Product.NovelName,
				MaskedDisplay: unit.MaskedDisplay,
			}
			mu.Unlock()
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	result := rp.executeUnit(ctx, unit)
	mu.Lock()
	(*results)[index] = result
	mu.Unlock()

	return nil
}

func (rp *RealPublisher) executeUnit(ctx context.Context, unit *PublishUnit) PublishResult {
	if unit.Credentials == "" && unit.ErrorCode != "" {
		msg := unit.ErrorMsg
		if msg == "" {
			msg = unit.ErrorCode
		}
		return PublishResult{
			AccountID:     unit.AccountID,
			UID:           unit.UID,
			Platform:      unit.Platform,
			Status:        "fail",
			ErrorCode:     unit.ErrorCode,
			ErrorMessage:  msg,
			NovelName:     unit.Product.NovelName,
			MaskedDisplay: unit.MaskedDisplay,
		}
	}

	adapter, ok := rp.adapters[unit.Platform]
	if !ok {
		return PublishResult{
			AccountID: unit.AccountID, UID: unit.UID, Platform: unit.Platform,
			Status: "fail", ErrorCode: ErrCodeUnsupportedPlatform,
			NovelName:     unit.Product.NovelName,
			MaskedDisplay: unit.MaskedDisplay,
		}
	}

	result := adapter.Publish(ctx, unit.Product, unit.Credentials, unit.MaskedDisplay)
	result.UID = unit.UID
	result.AccountID = unit.AccountID
	result.NovelName = unit.Product.NovelName

	logInfo("publish completed",
		"task_id", unit.TaskID,
		"account_id", unit.AccountID,
		"platform", unit.Platform,
		"status", result.Status,
		"post_id", result.PostID,
		"error_code", result.ErrorCode,
		"error_msg", result.ErrorMessage,
	)

	return *result
}
