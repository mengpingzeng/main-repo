// Package c1_publisher 提供番茄小说 PublishAdapter 实现。
// 番茄小说无公开 API，采用 Puppeteer 浏览器自动化发布章节。
package c1_publisher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// FanqiePublishAdapter 番茄小说平台发布适配器。
// 通过调用 Puppeteer 脚本（scripts/publish_fanqie.js）实现浏览器自动化发布。
// Cookie 通过环境变量 FANQIE_COOKIE 传入脚本，不经过命令行。
type FanqiePublishAdapter struct {
	scriptPath string
	nodeBin    string
	timeout    time.Duration
}

// NewFanqiePublishAdapter 创建番茄小说适配器（使用通用 AdapterConfig）。
func NewFanqiePublishAdapter(cfg AdapterConfig) *FanqiePublishAdapter {
	if cfg.NodeBin == "" {
		cfg.NodeBin = "node"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 1800 * time.Second
	}
	return &FanqiePublishAdapter{
		scriptPath: cfg.ScriptPath,
		nodeBin:    cfg.NodeBin,
		timeout:    cfg.Timeout,
	}
}

func (a *FanqiePublishAdapter) Platform() string {
	return "fanqie"
}

const fanqieMinContentLen = 1000

func (a *FanqiePublishAdapter) CheckInput(product ProductContent) string {
	title := product.Title
	if title == "" {
		title = firstLine(product.Text, 50)
	}
	if title == "" || title == "章节" {
		return "fanqie: chapter title is required"
	}
	if product.Text == "" {
		return "fanqie: text is empty"
	}
	textLen := len([]rune(product.Text))
	if textLen < fanqieMinContentLen {
		return fmt.Sprintf("fanqie: content too short (%d < %d)", textLen, fanqieMinContentLen)
	}
	if textLen > 20000 {
		return fmt.Sprintf("fanqie: text too long (%d > 20000)", textLen)
	}
	return ""
}

// --- Input/Output 结构 ---

type fanqieInput struct {
	Action        string `json:"action,omitempty"`
	Title         string `json:"title,omitempty"`
	Content       string `json:"content,omitempty"`
	NovelName     string `json:"novelName,omitempty"`
	VolumeName    string `json:"volumeName,omitempty"`
	VolumeID      string `json:"volumeId,omitempty"`
	ChapterNumber int    `json:"chapterNumber,omitempty"`
	DraftTitle    string `json:"draftTitle,omitempty"`
	BookID        string `json:"bookId,omitempty"`
	WorkID        string `json:"workId,omitempty"`
	DraftItemID   string `json:"draftItemId,omitempty"`
	Name          string `json:"name,omitempty"`
	Description   string `json:"description,omitempty"`
	Category      string `json:"category,omitempty"`
	CoverBase64   string `json:"coverBase64,omitempty"`
	Roles         string `json:"roles,omitempty"`
	AltNames      []string `json:"altNames,omitempty"`
}

type fanqieOutput struct {
	Success    bool                 `json:"success"`
	PostID     string               `json:"postId"`
	DraftItemID string              `json:"draftItemId"`
	Error      string               `json:"error"`
	ErrorCode  string               `json:"errorCode"`
	Action     string               `json:"action"`
	WorkID     string               `json:"workId"`
	NewlyCreated bool               `json:"newlyCreated"`
	MsToken    string               `json:"msToken"`
	Drafts     []FanqieDraftInfo    `json:"drafts"`
	PublishedChapters []FanqieChapterInfo `json:"publishedChapters"`
	LastPublished *FanqieLastPublished `json:"lastPublished"`
	Volumes    []FanqieVolumeInfo   `json:"volumes"`
	BookName   string               `json:"bookName"`
}

type FanqieDraftInfo struct {
	Title         string `json:"title"`
	ChapterNumber int    `json:"chapterNumber"`
	ItemID        string `json:"itemId"`
}

type FanqieChapterInfo struct {
	Title         string `json:"title"`
	ChapterNumber int    `json:"chapterNumber"`
	IsPublished   bool   `json:"isPublished"`
}

type FanqieLastPublished struct {
	ChapterNumber int    `json:"chapterNumber"`
	Title         string `json:"title"`
}

type FanqieVolumeInfo struct {
	VolumeName string `json:"volumeName"`
	VolumeID   string `json:"volumeId"`
}

type PlatformInfo struct {
	WorkID            string
	NewlyCreated      bool
	BookName          string
	MsToken           string
	Drafts            []FanqieDraftInfo
	PublishedChapters []FanqieChapterInfo
	LastPublished     *FanqieLastPublished
	Volumes           []FanqieVolumeInfo
}

// --- Core Methods ---

// Publish 调用 Puppeteer 脚本发布一章番茄小说（原有直接发布流程）。
func (a *FanqiePublishAdapter) Publish(ctx context.Context, product ProductContent, credentials string, maskedDisplay string) *PublishResult {
	if credentials == "" {
		return a.fail(ErrCodeCredentialFailed, "fanqie cookie is empty", maskedDisplay)
	}

	title := product.Title
	if title == "" {
		title = firstLine(product.Text, 50)
	}
	novelName := sanitizeNovelName(product.NovelName)
	if novelName == "" {
		novelName = "未命名作品"
	}
	volumeName := product.VolumeName
	if volumeName == "" {
		volumeName = "第一卷"
	}
	input := fanqieInput{
		Action:        "publish",
		Title:         title,
		Content:       product.Text,
		NovelName:     novelName,
		VolumeName:    volumeName,
		ChapterNumber: product.ChapterNumber,
	}
	return a.runScript(ctx, input, credentials, maskedDisplay)
}

// SaveDraft 将章节内容保存到番茄小说草稿箱。
func (a *FanqiePublishAdapter) SaveDraft(ctx context.Context, title, content, novelName string, chapterNumber int, credentials string, workId string) *PublishResult {
	if credentials == "" {
		return a.fail(ErrCodeCredentialFailed, "fanqie cookie is empty", "")
	}

	if content == "" {
		return a.fail(ErrCodeInputInvalid, "fanqie: content is empty", "")
	}
	if novelName == "" {
		return a.fail(ErrCodeInputInvalid, "fanqie: novelName is empty", "")
	}
	novelName = sanitizeNovelName(novelName)
	if title == "" {
		title = firstLine(content, 50)
	}

	input := fanqieInput{
		Action:        "save_draft",
		Title:         title,
		Content:       content,
		NovelName:     novelName,
		ChapterNumber: chapterNumber,
		WorkID:        workId,
	}
	return a.runScript(ctx, input, credentials, novelName)
}

// PublishDraft 从番茄小说草稿箱中发布指定草稿。
// draftTitle 用于在草稿箱中匹配目标草稿，volumeName 指定发布到哪个分卷。
func (a *FanqiePublishAdapter) PublishDraft(ctx context.Context, draftTitle, novelName, volumeName, credentials string, workId string, itemId string) *PublishResult {
	if credentials == "" {
		return a.fail(ErrCodeCredentialFailed, "fanqie cookie is empty", "")
	}
	if novelName == "" {
		return a.fail(ErrCodeInputInvalid, "fanqie: novelName is empty", "")
	}
	novelName = sanitizeNovelName(novelName)
	if volumeName == "" {
		volumeName = "第一卷"
	}

	input := fanqieInput{
		Action:      "publish_draft",
		DraftTitle:  draftTitle,
		NovelName:   novelName,
		VolumeName:  volumeName,
		WorkID:      workId,
		DraftItemID: itemId,
	}
	return a.runScript(ctx, input, credentials, novelName)
}

// PublishDraftViaPageAPI 通过 Puppeteer 脚本在浏览器内调用 publish_article API 发布草稿（不操作 DOM）。
func (a *FanqiePublishAdapter) PublishDraftViaPageAPI(ctx context.Context, workId, itemId, title, content, volumeName, volumeId, credentials string) *PublishResult {
	if credentials == "" {
		return a.fail(ErrCodeCredentialFailed, "fanqie cookie is empty", "")
	}
	if workId == "" {
		return a.fail(ErrCodeInputInvalid, "fanqie: workId is empty", "")
	}

	input := fanqieInput{
		Action:      "publish_article",
		Title:       title,
		Content:     content,
		VolumeName:  volumeName,
		VolumeID:    volumeId,
		WorkID:      workId,
		DraftItemID: itemId,
		NovelName:   volumeName,
	}
	return a.runScript(ctx, input, credentials, "")
}

// SaveDraftViaPageAPI 通过 Puppeteer 脚本在浏览器内调用 cover_article API 保存草稿（不操作 DOM）。
// cover_article API 优先，Puppeteer DOM 操作作为兜底。
func (a *FanqiePublishAdapter) SaveDraftViaPageAPI(ctx context.Context, title, content, novelName string, chapterNumber int, credentials, workId, volumeName, volumeId string) *PublishResult {
	if credentials == "" {
		return a.fail(ErrCodeCredentialFailed, "fanqie cookie is empty", "")
	}
	if content == "" {
		return a.fail(ErrCodeInputInvalid, "fanqie: content is empty", "")
	}
	if workId == "" {
		return a.fail(ErrCodeInputInvalid, "fanqie: workId is empty", "")
	}

	novelName = sanitizeNovelName(novelName)

	input := fanqieInput{
		Action:        "save_draft_api",
		Title:         title,
		Content:       content,
		NovelName:     novelName,
		ChapterNumber: chapterNumber,
		WorkID:        workId,
		VolumeName:    volumeName,
		VolumeID:      volumeId,
	}
	return a.runScript(ctx, input, credentials, novelName)
}

// GetPlatformInfo 获取番茄小说平台的草稿箱、已发布章节、分卷信息。
// 用于发布前状态检查和发布后验证。
func (a *FanqiePublishAdapter) GetPlatformInfo(ctx context.Context, novelName, credentials string, workId string, altNames []string) (*PlatformInfo, *PublishResult) {
	if credentials == "" {
		return nil, a.fail(ErrCodeCredentialFailed, "fanqie cookie is empty", "")
	}
	if novelName == "" {
		return nil, a.fail(ErrCodeInputInvalid, "fanqie: novelName is empty", "")
	}
	novelName = sanitizeNovelName(novelName)

	input := fanqieInput{
		Action:    "get_platform_info",
		NovelName: novelName,
		WorkID:    workId,
		AltNames:  altNames,
	}

	authorName, resolveErr := a.ResolveAuthorName(ctx, credentials)
	if resolveErr != nil {
		log.Printf("[fanqie] ResolveAuthorName failed: %v", resolveErr)
	} else {
		log.Printf("[fanqie] GetPlatformInfo author=%s novel=%s", authorName, novelName)
	}

	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, a.fail(ErrCodeBuildRequest, "marshal input failed: "+err.Error(), "")
	}

	output, rawOutput, err := a.execScript(ctx, inputBytes, credentials)
	if err != nil {
		return nil, a.fail(ErrCodePlatformError, err.Error(), "")
	}

	if !output.Success {
		code := classifyFanqieError(output.Error)
		if output.ErrorCode != "" {
			code = output.ErrorCode
		}
		return nil, a.fail(code, output.Error, "")
	}

	info := &PlatformInfo{
		WorkID:            output.WorkID,
		NewlyCreated:      output.NewlyCreated,
		BookName:          output.BookName,
		MsToken:           output.MsToken,
		Drafts:            output.Drafts,
		PublishedChapters: output.PublishedChapters,
		LastPublished:     output.LastPublished,
		Volumes:           output.Volumes,
	}

	rawLog := rawOutput
	if len(rawLog) > 2000 {
		rawLog = rawLog[len(rawLog)-2000:]
	}
	log.Printf("[fanqie] GetPlatformInfo: novel=%s workId=%s drafts=%d chapters=%d volumes=%d",
		novelName, info.WorkID, len(info.Drafts), len(info.PublishedChapters), len(info.Volumes))

	return info, nil
}

// ResolveAuthorName 从 Fanqie Cookie 解析作者笔名。
func (a *FanqiePublishAdapter) ResolveAuthorName(ctx context.Context, cookie string) (string, error) {
	fanqieCheckURL := "https://fanqienovel.com/api/author/account/info/v0/?aid=2503&app_name=muye_novel"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fanqieCheckURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://fanqienovel.com/main/writer/")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			AuthorName string `json:"author_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("api code=%d", result.Code)
	}
	authorName := strings.TrimSpace(result.Data.AuthorName)
	if authorName == "" {
		return "", fmt.Errorf("author_name is empty")
	}
	log.Printf("[fanqie] ResolveAuthorName: %s", authorName)
	return authorName, nil
}

// SetBookInfo 上传封面并设置书籍信息（仅番茄平台新书创建后调用）。
func (a *FanqiePublishAdapter) SetBookInfo(ctx context.Context, cookie, workId, name, description, category, roles string, coverBytes []byte) *PublishResult {
	name = sanitizeNovelName(name)
	input := fanqieInput{
		Action:      "set_book_info",
		Name:        name,
		Description: description,
		Category:    category,
		CoverBase64: base64.StdEncoding.EncodeToString(coverBytes),
		WorkID:      workId,
		Roles:       roles,
	}
	return a.runScript(ctx, input, cookie, name)
}

var msTokenRe = regexp.MustCompile(`msToken=([^;]+)`)

func extractMsToken(cookie string) string {
	m := msTokenRe.FindStringSubmatch(cookie)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func uploadCoverImage(ctx context.Context, cookie, msToken string, coverBytes []byte) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("upfile", "cover.png")
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(coverBytes); err != nil {
		return "", fmt.Errorf("write cover data: %w", err)
	}
	writer.Close()

	uploadURL := "https://fanqienovel.com/api/author/data/upload_pic_v1/v0?msToken=" + msToken
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &buf)
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://fanqienovel.com/main/writer/")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32768))
	if err != nil {
		return "", fmt.Errorf("read upload response: %w", err)
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			PicURI string `json:"pic_uri"`
			PicURL string `json:"pic_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse upload response: %w, body=%s", err, string(body))
	}
	if result.Code != 0 {
		return "", fmt.Errorf("upload failed: code=%d msg=%s", result.Code, result.Message)
	}
	if result.Data.PicURI == "" {
		return "", fmt.Errorf("upload response missing pic_uri")
	}
	return result.Data.PicURI, nil
}

func modifyBookInfo(ctx context.Context, cookie, msToken, name, description, category, picURI string) error {
	body := map[string]interface{}{
		"book_name":   name,
		"description": description,
		"genre":       category,
		"pic_uri":     picURI,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal modify body: %w", err)
	}

	modifyURL := "https://fanqienovel.com/api/author/book/modify_book/v0?msToken=" + msToken
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, modifyURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create modify request: %w", err)
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://fanqienovel.com/main/writer/")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("modify request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 16384))
	if err != nil {
		return fmt.Errorf("read modify response: %w", err)
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("parse modify response: %w, body=%s", err, string(respBody))
	}
	if result.Code != 0 {
		return fmt.Errorf("modify failed: code=%d msg=%s", result.Code, result.Message)
	}
	return nil
}

// --- 脚本执行 ---

func (a *FanqiePublishAdapter) runScript(ctx context.Context, input fanqieInput, credentials string, maskedDisplay string) *PublishResult {
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return a.fail(ErrCodeBuildRequest, "marshal input failed: "+err.Error(), maskedDisplay)
	}

	output, _, err := a.execScript(ctx, inputBytes, credentials)
	if err != nil {
		return a.fail(ErrCodePlatformError, err.Error(), maskedDisplay)
	}

	if !output.Success {
		code := classifyFanqieError(output.Error)
		if output.ErrorCode != "" {
			code = output.ErrorCode
		}
		return a.fail(code, output.Error, maskedDisplay)
	}

	return &PublishResult{
		Platform:      "fanqie",
		Status:        "ok",
		PostID:        output.PostID,
		DraftItemID:   output.DraftItemID,
		MaskedDisplay: maskedDisplay,
	}
}

func (a *FanqiePublishAdapter) execScript(ctx context.Context, inputBytes []byte, credentials string) (*fanqieOutput, string, error) {
	inputBase64 := base64.StdEncoding.EncodeToString(inputBytes)

	execCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, a.nodeBin, a.scriptPath, "--base64")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	cmd.Stdin = strings.NewReader(inputBase64)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Env = append(os.Environ(), "FANQIE_COOKIE="+credentials)

	h := sha256.Sum256([]byte(credentials))
	log.Printf("[fanqie] input: action=%s cookie_len=%d cookie_sha256=%s", extractAction(inputBytes), len(credentials), hex.EncodeToString(h[:4]))

	err := cmd.Run()

	stderrStr := stderr.String()
	stdoutStr := strings.TrimSpace(stdout.String())

	if len(stderrStr) > 0 {
		var stderrLog string
		if len(stderrStr) > 2000 {
			stderrLog = stderrStr[len(stderrStr)-2000:]
		} else {
			stderrLog = stderrStr
		}
		log.Printf("[fanqie] script log: %s", stderrLog)
	}

	if stdoutStr != "" {
		var output fanqieOutput
		if json.Unmarshal([]byte(stdoutStr), &output) == nil {
			return &output, stderrStr, nil
		}
	}

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return nil, stderrStr, fmt.Errorf("puppeteer script timeout after %v", a.timeout)
		}
		return nil, stderrStr, fmt.Errorf("puppeteer script failed: %v (stderr: %s)", err, truncateStr(stderrStr, 200))
	}

	return nil, stderrStr, fmt.Errorf("puppeteer script returned empty output")
}

func extractAction(inputBytes []byte) string {
	var input fanqieInput
	if json.Unmarshal(inputBytes, &input) == nil {
		act := input.Action
		if act == "" {
			act = "publish"
		}
		return act
	}
	return "unknown"
}

// --- 辅助函数 ---

func (a *FanqiePublishAdapter) fail(code, msg string, maskedDisplay string) *PublishResult {
	return &PublishResult{
		Platform:      "fanqie",
		Status:        "fail",
		ErrorCode:     code,
		ErrorMessage:  msg,
		MaskedDisplay: maskedDisplay,
	}
}

func firstLine(text string, maxRunes int) string {
	lines := strings.SplitN(text, "\n", 2)
	if len(lines) == 0 {
		return ""
	}
	first := strings.TrimSpace(lines[0])
	runes := []rune(first)
	if len(runes) <= maxRunes {
		return first
	}
	return string(runes[:maxRunes])
}

func classifyFanqieError(errMsg string) string {
	lower := strings.ToLower(errMsg)
	switch {
	case strings.Contains(lower, "daily_limit") || strings.Contains(lower, "上限") || strings.Contains(lower, "超限"):
		return ErrCodeDailyLimit
	case strings.Contains(lower, "cookie") || strings.Contains(lower, "expired") || strings.Contains(lower, "login"):
		return ErrCodeAccount401
	case strings.Contains(lower, "timeout"):
		return ErrCodeAPITimeout
	case strings.Contains(lower, "not found") || strings.Contains(lower, "not set"):
		return ErrCodeCredentialFailed
	default:
		return ErrCodePlatformError
	}
}

func sanitizeNovelName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || (r >= 0x4e00 && r <= 0x9fff) ||
			r == ' ' || r == '_' {
			b.WriteRune(r)
		}
	}
	cleaned := strings.TrimSpace(b.String())
	if cleaned == "" {
		cleaned = "未命名作品"
	}
	return cleaned
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
