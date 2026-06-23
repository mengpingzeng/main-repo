// Package c1_publisher 提供七猫小说 PublishAdapter 实现。
// 七猫小说采用 Puppeteer 浏览器自动化 + fetch API 方式发布章节。
package c1_publisher

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// QimaoPublishAdapter 七猫小说平台发布适配器。
type QimaoPublishAdapter struct {
	scriptPath string
	nodeBin    string
	timeout    time.Duration
}

// NewQimaoPublishAdapter 创建七猫小说适配器。
func NewQimaoPublishAdapter(cfg AdapterConfig) *QimaoPublishAdapter {
	if cfg.NodeBin == "" {
		cfg.NodeBin = "node"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 600 * time.Second
	}
	return &QimaoPublishAdapter{
		scriptPath: cfg.ScriptPath,
		nodeBin:    cfg.NodeBin,
		timeout:    cfg.Timeout,
	}
}

func (a *QimaoPublishAdapter) Platform() string {
	return "qimao"
}

const qimaoMinContentLen = 1000

func (a *QimaoPublishAdapter) CheckInput(product ProductContent) string {
	title := product.Title
	if title == "" {
		title = firstLine(product.Text, 50)
	}
	if title == "" {
		return "qimao: chapter title is required"
	}
	if product.Text == "" {
		return "qimao: text is empty"
	}
	textLen := len([]rune(product.Text))
	if textLen < qimaoMinContentLen {
		return fmt.Sprintf("qimao: content too short (%d < %d)", textLen, qimaoMinContentLen)
	}
	return ""
}

// --- 七猫数据结构 ---

type qimaoInput struct {
	Action        string `json:"action,omitempty"`
	NovelName     string `json:"novelName,omitempty"`
	Title         string `json:"title,omitempty"`
	Content       string `json:"content,omitempty"`
	BookID        string `json:"bookId,omitempty"`
	ChapterID     string `json:"chapterId,omitempty"`
	ChapterNumber int    `json:"chapterNumber,omitempty"`
	BookDesc      string `json:"bookDesc,omitempty"`
	Characters    string `json:"characters,omitempty"`
	Category1     string `json:"category1,omitempty"`
	Category2     string `json:"category2,omitempty"`
	TagIds        string `json:"tagIds,omitempty"`
	CoverNum      int    `json:"coverNum,omitempty"`
	ClientID      string `json:"clientId,omitempty"`
	CoverBase64   string `json:"coverBase64,omitempty"`
}

// QimaoOutput Puppeteer 脚本返回的通用输出结构。
type QimaoOutput struct {
	Success       bool                 `json:"success"`
	Action        string               `json:"action"`
	Error         string               `json:"error"`
	BookID        string               `json:"bookId"`
	BookName      string               `json:"bookName"`
	BookExists    bool                 `json:"bookExists"`
	MaxNameIndex  int                  `json:"maxNameIndex"`
	ChapterID     string               `json:"chapterId"`
	Books         []QimaoBookInfo      `json:"books"`
	Chapters      []QimaoChapterInfo   `json:"chapters"`
	LastPublished *QimaoChapterInfo    `json:"lastPublished"`
	CategoryList  []QimaoCategoryGroup `json:"categoryList"`
	TagList       []QimaoTagGroup      `json:"tagList"`
	Category1     string               `json:"category1"`
	Category2     string               `json:"category2"`
	PickedTagIds  string               `json:"pickedTagIds"`
}

type QimaoBookInfo struct {
	BookID string `json:"book_id"`
	Title  string `json:"title"`
}

type QimaoChapterInfo struct {
	BookID      string `json:"book_id"`
	ChapterID   string `json:"chapter_id"`
	ChapterName string `json:"chapter_name"`
	NameIndex   string `json:"name_index"`
	PublishType string `json:"publish_type"`
	VolumeID    string `json:"volume_id"`
	StatusName  string `json:"status_name"`
	DraftSort   int    `json:"draft_sort"`
}

type QimaoCategoryGroup struct {
	Category []QimaoCategoryItem `json:"category"`
}

type QimaoCategoryItem struct {
	ID           string               `json:"id"`
	CategoryName string               `json:"category_name"`
	Children     []QimaoCategoryChild `json:"children"`
}

type QimaoCategoryChild struct {
	ID           string `json:"id"`
	CategoryName string `json:"category_name"`
}

type QimaoTagGroup struct {
	TypeID         int                  `json:"type_id"`
	TypeName       string               `json:"type_name"`
	CanChooseCount int                  `json:"can_choose_count"`
	SelectList     []QimaoTagSelectItem `json:"select_list"`
}

type QimaoTagSelectItem struct {
	ID      string `json:"id"`
	TagID   string `json:"tag_id"`
	TagName string `json:"tag_name"`
}

// --- 公开方法 ---

// Publish 调用 Puppeteer 脚本发布一章七猫小说。
func (a *QimaoPublishAdapter) Publish(ctx context.Context, product ProductContent, credentials string, maskedDisplay string) *PublishResult {
	if credentials == "" {
		return a.fail(ErrCodeCredentialFailed, "qimao cookie is empty", maskedDisplay)
	}
	return a.fail(ErrCodePlatformError, "use SaveDraft + PublishDraft for qimao", maskedDisplay)
}

// GetPlatformInfo 获取七猫平台作品列表 + 建书选项。
func (a *QimaoPublishAdapter) GetPlatformInfo(ctx context.Context, novelName, credentials, bookId string) (*QimaoOutput, *PublishResult) {
	if credentials == "" {
		return nil, a.fail(ErrCodeCredentialFailed, "qimao cookie is empty", "")
	}
	novelName = sanitizeNovelName(novelName)
	input := qimaoInput{Action: "get_platform_info", NovelName: novelName, BookID: bookId}
	return a.execAndParse(ctx, input, credentials)
}

// GetBookOption 获取七猫建书选项。
func (a *QimaoPublishAdapter) GetBookOption(ctx context.Context, credentials string) (*QimaoOutput, *PublishResult) {
	if credentials == "" {
		return nil, a.fail(ErrCodeCredentialFailed, "qimao cookie is empty", "")
	}
	input := qimaoInput{Action: "get_book_option"}
	return a.execAndParse(ctx, input, credentials)
}

// ResolveAuthorName 从七猫 Cookie 解析作者笔名。
func (a *QimaoPublishAdapter) ResolveAuthorName(ctx context.Context, cookie string) (string, error) {
	qimaoProfileURL := "https://zuozhe.qimao.com/api/author/profile"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, qimaoProfileURL, nil)
	if err != nil {
		return "", fmt.Errorf("build qimao profile request: %w", err)
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://zuozhe.qimao.com/front/index")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("qimao profile request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("qimao profile http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("read qimao profile response: %w", err)
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			User struct {
				PenName string `json:"pen_name"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse qimao profile response: %w", err)
	}
	if result.Code != 200 {
		return "", fmt.Errorf("qimao profile code=%d", result.Code)
	}

	authorName := strings.TrimSpace(result.Data.User.PenName)
	if authorName == "" {
		return "", fmt.Errorf("qimao profile: pen_name is empty")
	}
	log.Printf("[qimao] ResolveAuthorName: %s", authorName)
	return authorName, nil
}

// CreateBook 在七猫平台创建新作品。tags 可为 nil。bookOpt 包含分类和标签选项。
// description 作品简介，characters 主角名/角色信息。
func (a *QimaoPublishAdapter) CreateBook(ctx context.Context, credentials, novelName, description, characters string, tags []string, bookOpt *QimaoOutput) *PublishResult {
	if credentials == "" {
		return a.fail(ErrCodeCredentialFailed, "qimao cookie is empty", "")
	}
	if novelName == "" {
		return a.fail(ErrCodeInputInvalid, "qimao: novelName is empty", "")
	}
	novelName = sanitizeNovelName(novelName)

	category1 := ""
	category2 := ""
	tagIds := ""
	if bookOpt != nil {
		category1 = bookOpt.Category1
		category2 = bookOpt.Category2
		tagIds = bookOpt.PickedTagIds
	}
	if category1 == "" {
		category1 = "301"
	}
	if category2 == "" {
		category2 = "333"
	}
	if tagIds == "" {
		tagIds = "1,28,47"
	}

	input := qimaoInput{
		Action:     "create_book",
		Title:      novelName,
		BookDesc:   description,
		Characters: characters,
		Category1:  category1,
		Category2:  category2,
		TagIds:     tagIds,
		CoverNum:   5,
		ClientID:   "1",
	}

	a.logRolesTrace("CreateBook.input", fmt.Sprintf("novelName=%q characters(len=%d)=%q description(len=%d) cat1=%s cat2=%s", novelName, len(characters), characters, len(description), category1, category2))

	output, err := a.runScript(ctx, input, credentials)
	if err != nil {
		return a.fail(ErrCodePlatformError, err.Error(), "")
	}
	if !output.Success {
		return a.fail(ErrCodePlatformError, output.Error, "")
	}

	log.Printf("[qimao] CreateBook success: bookId=%s novelName=%s", output.BookID, novelName)
	return &PublishResult{Status: "ok", PostID: output.BookID, NovelName: novelName}
}

// SetBookInfo 上传封面并设置书籍信息（仅七猫平台新书创建后调用）。
func (a *QimaoPublishAdapter) SetBookInfo(ctx context.Context, cookie, bookId, name, description, category1, category2, tagIds, roles string, coverBytes []byte) *PublishResult {
	if len(coverBytes) == 0 {
		return a.fail(ErrCodeInputInvalid, "qimao: coverBytes is empty", "")
	}
	name = sanitizeNovelName(name)
	input := qimaoInput{
		Action:      "set_book_info",
		BookID:      bookId,
		Title:       name,
		BookDesc:    description,
		Category1:   category1,
		Category2:   category2,
		TagIds:      tagIds,
		Characters:  roles,
		CoverBase64: base64.StdEncoding.EncodeToString(coverBytes),
	}
	output, err := a.runScript(ctx, input, cookie)
	if err != nil {
		return a.fail(ErrCodePlatformError, err.Error(), "")
	}
	if !output.Success {
		return a.fail(ErrCodePlatformError, output.Error, "")
	}
	log.Printf("[qimao] SetBookInfo success: bookId=%s name=%s", bookId, name)
	return &PublishResult{Status: "ok"}
}

// SaveDraft 将章节内容保存到七猫草稿箱（publish_type=3）。
func (a *QimaoPublishAdapter) SaveDraft(ctx context.Context, bookID, title, content string, chapterNumber int, credentials string) *PublishResult {
	if credentials == "" {
		return a.fail(ErrCodeCredentialFailed, "qimao cookie is empty", "")
	}
	if bookID == "" {
		return a.fail(ErrCodeInputInvalid, "qimao: bookId is empty", "")
	}
	if content == "" {
		return a.fail(ErrCodeInputInvalid, "qimao: content is empty", "")
	}

	input := qimaoInput{
		Action:        "save_draft",
		BookID:        bookID,
		Title:         title,
		Content:       content,
		ChapterNumber: chapterNumber,
	}

	output, err := a.runScript(ctx, input, credentials)
	if err != nil {
		return a.fail(ErrCodePlatformError, err.Error(), "")
	}
	if !output.Success {
		return a.fail(ErrCodePlatformError, output.Error, "")
	}

	log.Printf("[qimao] SaveDraft success: bookId=%s title=%s", bookID, title)
	return &PublishResult{Status: "ok"}
}

// GetChapterList 获取七猫章节列表（已发布 + 草稿箱）。
func (a *QimaoPublishAdapter) GetChapterList(ctx context.Context, bookID, credentials string) (*QimaoOutput, *PublishResult) {
	if credentials == "" {
		return nil, a.fail(ErrCodeCredentialFailed, "qimao cookie is empty", "")
	}
	if bookID == "" {
		return nil, a.fail(ErrCodeInputInvalid, "qimao: bookId is empty", "")
	}
	input := qimaoInput{Action: "get_chapter_list", BookID: bookID}
	return a.execAndParse(ctx, input, credentials)
}

// PublishDraft 从七猫草稿箱发布指定章节。
func (a *QimaoPublishAdapter) PublishDraft(ctx context.Context, bookID, chapterID, credentials string) *PublishResult {
	if credentials == "" {
		return a.fail(ErrCodeCredentialFailed, "qimao cookie is empty", "")
	}
	if bookID == "" {
		return a.fail(ErrCodeInputInvalid, "qimao: bookId is empty", "")
	}
	if chapterID == "" {
		return a.fail(ErrCodeInputInvalid, "qimao: chapterId is empty", "")
	}

	input := qimaoInput{Action: "publish_draft", BookID: bookID, ChapterID: chapterID}
	output, err := a.runScript(ctx, input, credentials)
	if err != nil {
		return a.fail(ErrCodePlatformError, err.Error(), "")
	}
	if !output.Success {
		return a.fail(ErrCodePlatformError, output.Error, "")
	}

	log.Printf("[qimao] PublishDraft success: bookId=%s chapterId=%s", bookID, chapterID)
	return &PublishResult{Status: "ok", PostID: chapterID}
}

// MatchDraftChapterID 从章节列表中匹配要发布的草稿 chapter_id。
// 规则：已发布(publish_type=1)中 name_index 最大值 M+1 → 草稿箱中找同名 index。
func (a *QimaoPublishAdapter) MatchDraftChapterID(output *QimaoOutput, expectedChapter int) string {
	if output == nil || len(output.Chapters) == 0 {
		return ""
	}
	maxPublished := output.MaxNameIndex
	if expectedChapter > 0 && expectedChapter > maxPublished {
		maxPublished = expectedChapter - 1
	}
	nextIndex := maxPublished + 1

	for _, ch := range output.Chapters {
		if ch.PublishType == "0" {
			idx := parseIntSafe(ch.NameIndex)
			if idx == nextIndex {
				return ch.ChapterID
			}
		}
	}
	for _, ch := range output.Chapters {
		if ch.PublishType == "0" {
			log.Printf("[qimao] MatchDraftChapterID fallback: nameIndex=%s chapterId=%s", ch.NameIndex, ch.ChapterID)
			return ch.ChapterID
		}
	}
	return ""
}

// --- 内部辅助 ---

func (a *QimaoPublishAdapter) execAndParse(ctx context.Context, input qimaoInput, credentials string) (*QimaoOutput, *PublishResult) {
	output, err := a.runScript(ctx, input, credentials)
	catLen := 0
	tagLen := 0
	outSuccess := false
	outErr := ""
	if output != nil {
		catLen = len(output.CategoryList)
		tagLen = len(output.TagList)
		outSuccess = output.Success
		outErr = output.Error
	}
	a.logRolesTrace("execAndParse", fmt.Sprintf("action=%s runScriptErr=%v outputNil=%v outputSuccess=%v outputError=%q catListLen=%d tagListLen=%d",
		input.Action, err, output == nil, outSuccess, outErr, catLen, tagLen))
	if err != nil {
		return nil, a.fail(ErrCodePlatformError, err.Error(), "")
	}
	if !output.Success {
		return nil, a.fail(ErrCodePlatformError, output.Error, "")
	}
	return output, nil
}



// --- Puppeteer 执行 ---

func (a *QimaoPublishAdapter) runScript(ctx context.Context, input qimaoInput, credentials string) (*QimaoOutput, error) {
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}
	a.logRolesTrace("runScript.jsonMarshal", fmt.Sprintf("action=%s rawJSON=%s", input.Action, string(inputBytes)))
	encoded := base64.StdEncoding.EncodeToString(inputBytes)

	cmdCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, a.nodeBin, "--max-old-space-size=256", a.scriptPath, "--base64")
	cmd.Stdin = strings.NewReader(encoded)
	cmd.Env = append(os.Environ(), "QIMAO_COOKIE="+credentials)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	stderrStr := stderr.String()

	if stderrStr != "" {
		for _, line := range strings.Split(strings.TrimSpace(stderrStr), "\n") {
			log.Printf("[qimao_puppeteer] %s", line)
		}
	}

	stdoutStr := strings.TrimSpace(stdout.String())
	if stdoutStr != "" {
		var output QimaoOutput
		if err := json.Unmarshal([]byte(stdoutStr), &output); err == nil {
			return &output, nil
		}
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				return nil, fmt.Errorf("qimao puppeteer killed by signal %d", status.Signal())
			}
		}
		return nil, fmt.Errorf("qimao puppeteer failed: %w", err)
	}

	return nil, fmt.Errorf("qimao puppeteer empty output")
}

func (a *QimaoPublishAdapter) fail(code, msg string, maskedDisplay string) *PublishResult {
	return &PublishResult{
		Status:        "fail",
		ErrorCode:     code,
		ErrorMessage:  msg,
		MaskedDisplay: maskedDisplay,
	}
}

func parseIntSafe(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func (a *QimaoPublishAdapter) logRolesTrace(step, detail string) {
	f, err := os.OpenFile("/tmp/logs/roles_trace.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] step=%s %s\n", time.Now().Format("2006-01-02 15:04:05.000"), step, detail)
}
