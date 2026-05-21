// Package c1_publisher 提供逐浪网 PublishAdapter 实现。
// 逐浪网无公开 API，采用 Puppeteer 浏览器自动化发布章节。
package c1_publisher

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// ZhulangPublishAdapter 逐浪网平台发布适配器。
// 通过调用 Puppeteer 脚本（scripts/publish_zhulang.js）实现浏览器自动化发布。
// Cookie 通过环境变量 ZHULANG_COOKIE 传入脚本，不经过命令行。
type ZhulangPublishAdapter struct {
	scriptPath string // publish_zhulang.js 路径
	nodeBin    string // node 可执行文件路径（默认 "node"）
	timeout    time.Duration
}

// NewZhulangPublishAdapter 创建逐浪网适配器（使用通用 AdapterConfig）。
//
// cfg.ScriptPath 必填，指向 publish_zhulang.js 脚本。
// cfg.NodeBin 为空时默认 "node"，cfg.Timeout <= 0 时默认 300s。
func NewZhulangPublishAdapter(cfg AdapterConfig) *ZhulangPublishAdapter {
	if cfg.NodeBin == "" {
		cfg.NodeBin = "node"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 300 * time.Second
	}
	return &ZhulangPublishAdapter{
		scriptPath: cfg.ScriptPath,
		nodeBin:    cfg.NodeBin,
		timeout:    cfg.Timeout,
	}
}

func (a *ZhulangPublishAdapter) Platform() string {
	return "zhulang"
}

const zhulangMinContentLen = 1000

// CheckInput 逐浪网内容校验。
// 规则：作品名非空、章节名非空、正文非空、正文 >= 1000 字。
func (a *ZhulangPublishAdapter) CheckInput(product ProductContent) string {
	if product.NovelName == "" {
		return "zhulang: novelName is required"
	}
	title := product.Title
	if title == "" {
		title = firstLine(product.Text, 50)
	}
	if title == "" {
		return "zhulang: chapter title is required"
	}
	if product.Text == "" {
		return "zhulang: text is empty"
	}
	textLen := len([]rune(product.Text))
	if textLen < zhulangMinContentLen {
		return fmt.Sprintf("zhulang: content too short (%d < %d)", textLen, zhulangMinContentLen)
	}
	return ""
}

// zhulangInput 传入 Puppeteer 脚本的 JSON 结构。
type zhulangInput struct {
	Title           string `json:"title"`
	Content         string `json:"content"`
	NovelName       string `json:"novelName"`
	VolumeName      string `json:"volumeName"`
	ChapterNumber   int    `json:"chapterNumber"`
	ChapterType     string `json:"chapterType,omitempty"`
	PublishDirectly bool   `json:"publishDirectly,omitempty"`
	NovelGenre      string `json:"novelGenre,omitempty"`
	NovelSubGenre   string `json:"novelSubGenre,omitempty"`
}

// zhulangOutput Puppeteer 脚本的 JSON 输出。
type zhulangOutput struct {
	Success bool   `json:"success"`
	PostID  string `json:"postId"`
	Error   string `json:"error"`
}

// Publish 调用 Puppeteer 脚本发布一章逐浪网小说。
//
// credentials 是逐浪网的 Cookie 字符串，通过环境变量 ZHULANG_COOKIE 传入。
//
// 标题/作品/分卷/章节号 直接从 product 结构体读取（见 types.go ProductContent）。
func (a *ZhulangPublishAdapter) Publish(ctx context.Context, product ProductContent, credentials string, maskedDisplay string) *PublishResult {
	if credentials == "" {
		return a.fail(ErrCodeCredentialFailed, "zhulang cookie is empty", maskedDisplay)
	}

	title := product.Title
	if title == "" {
		title = firstLine(product.Text, 50)
	}
	novelName := product.NovelName
	if novelName == "" {
		novelName = "未命名作品"
	}
	volumeName := product.VolumeName
	if volumeName == "" {
		volumeName = "第一卷"
	}
	input := zhulangInput{
		Title:           title,
		Content:         product.Text,
		NovelName:       novelName,
		VolumeName:      volumeName,
		ChapterNumber:   product.ChapterNumber,
		PublishDirectly: true,
	}

	inputBytes, err := json.Marshal(input)
	if err != nil {
		return a.fail(ErrCodeBuildRequest, "marshal input failed: "+err.Error(), maskedDisplay)
	}

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

	cmd.Env = append(os.Environ(), "ZHULANG_COOKIE="+credentials)

	log.Printf("[zhulang] input JSON: %s", string(inputBytes))

	err = cmd.Run()

	stderrStr := stderr.String()
	stdoutStr := strings.TrimSpace(stdout.String())

	if stdoutStr != "" {
		var output zhulangOutput
		if json.Unmarshal([]byte(stdoutStr), &output) == nil {
			if output.Success {
				return &PublishResult{
					Platform:      "zhulang",
					Status:        "ok",
					PostID:        output.PostID,
					MaskedDisplay: maskedDisplay,
				}
			}
			return a.fail(classifyZhulangError(output.Error), output.Error, maskedDisplay)
		}
	}

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return a.fail(ErrCodeAPITimeout, fmt.Sprintf("puppeteer script timeout after %v", a.timeout), maskedDisplay)
		}
		return a.fail(ErrCodePlatformError,
			fmt.Sprintf("puppeteer script failed: %v (stderr: %s)", err, truncateStr(stderrStr, 200)), maskedDisplay)
	}

	return a.fail(ErrCodePlatformError, "puppeteer script returned empty output", maskedDisplay)
}

func (a *ZhulangPublishAdapter) fail(code, msg string, maskedDisplay string) *PublishResult {
	return &PublishResult{
		Platform:      "zhulang",
		Status:        "fail",
		ErrorCode:     code,
		ErrorMessage:  msg,
		MaskedDisplay: maskedDisplay,
	}
}

func classifyZhulangError(errMsg string) string {
	lower := strings.ToLower(errMsg)
	switch {
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
