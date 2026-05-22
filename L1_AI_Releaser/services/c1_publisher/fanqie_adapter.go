// Package c1_publisher 提供番茄小说 PublishAdapter 实现。
// 番茄小说无公开 API，采用 Puppeteer 浏览器自动化发布章节。
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

// FanqiePublishAdapter 番茄小说平台发布适配器。
// 通过调用 Puppeteer 脚本（scripts/publish_fanqie.js）实现浏览器自动化发布。
// Cookie 通过环境变量 FANQIE_COOKIE 传入脚本，不经过命令行。
type FanqiePublishAdapter struct {
	scriptPath string // publish_fanqie.js 路径
	nodeBin    string // node 可执行文件路径（默认 "node"）
	timeout    time.Duration
}

// NewFanqiePublishAdapter 创建番茄小说适配器（使用通用 AdapterConfig）。
//
// cfg.ScriptPath 必填，指向 publish_fanqie.js 脚本。
// cfg.NodeBin 为空时默认 "node"，cfg.Timeout <= 0 时默认 90s。
func NewFanqiePublishAdapter(cfg AdapterConfig) *FanqiePublishAdapter {
	if cfg.NodeBin == "" {
		cfg.NodeBin = "node"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 300 * time.Second
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

const fanqieMinContentLen = 1000 // 番茄小说正文最低 1000 字

// CheckInput 番茄小说内容校验。
// 规则：标题非空、正文非空、正文 ≥ 1000 字、正文字数 ≤ 20000 字。
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

// fanqieInput 传入 Puppeteer 脚本的 JSON 结构。
type fanqieInput struct {
	Title         string `json:"title"`
	Content       string `json:"content"`
	NovelName     string `json:"novelName"`
	VolumeName    string `json:"volumeName"`
	ChapterNumber int    `json:"chapterNumber"`
}

// fanqieOutput Puppeteer 脚本的 JSON 输出。
type fanqieOutput struct {
	Success bool   `json:"success"`
	PostID  string `json:"postId"`
	Error   string `json:"error"`
}

// Publish 调用 Puppeteer 脚本发布一章番茄小说。
//
// credentials 是番茄小说的 Cookie 字符串（如 "sessionid=xxx; ..."），
// 通过环境变量 FANQIE_COOKIE 传入 Node.js 脚本，不经过命令行。
//
// 标题/作品/分卷/章节号 直接从 product 结构体读取（见 types.go ProductContent）。
func (a *FanqiePublishAdapter) Publish(ctx context.Context, product ProductContent, credentials string, maskedDisplay string) *PublishResult {
	if credentials == "" {
		return a.fail(ErrCodeCredentialFailed, "fanqie cookie is empty", maskedDisplay)
	}

	// 清理上次可能残留的 Chrome 进程，防止 OOM
	exec.Command("pkill", "-f", "chrome.*headless").Run()
	exec.Command("pkill", "-f", "chrome_crashpad").Run()

	// 从 ProductContent 中读取番茄小说专用字段（带默认值回退）
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
	input := fanqieInput{
		Title:         title,
		Content:       product.Text,
		NovelName:     novelName,
		VolumeName:    volumeName,
		ChapterNumber: product.ChapterNumber,
	}

	inputBytes, err := json.Marshal(input)
	if err != nil {
		return a.fail(ErrCodeBuildRequest, "marshal input failed: "+err.Error(), maskedDisplay)
	}

	// base64 编码，避免 JSON 中的换行/引号在命令行传递时被截断
	inputBase64 := base64.StdEncoding.EncodeToString(inputBytes)

	execCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	// 使用 --base64 模式：通过 stdin 管道传入 base64 数据，而非命令行参数
	cmd := exec.CommandContext(execCtx, a.nodeBin, a.scriptPath, "--base64")

	// 设置进程组，确保超时杀死 Node 时连带清理 Chrome 子进程
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

	log.Printf("[fanqie] input JSON: %s", string(inputBytes))
	log.Printf("[fanqie] cookie length=%d platform=%s", len(credentials), a.Platform())

	err = cmd.Run()

	stderrStr := stderr.String()
	stdoutStr := strings.TrimSpace(stdout.String())

	// 始终打印脚本日志，便于排查发布流程
	if len(stderrStr) > 0 {
		var stderrLog string
		if len(stderrStr) > 2000 {
			stderrLog = stderrStr[len(stderrStr)-2000:]
		} else {
			stderrLog = stderrStr
		}
		log.Printf("[fanqie] script log: %s", stderrLog)
	}

	// Try to parse stdout first, even on error — the script always writes
	// JSON to stdout (both success and failure), and its error messages
	// are more useful than generic "exit status 1".
	if stdoutStr != "" {
		var output fanqieOutput
		if json.Unmarshal([]byte(stdoutStr), &output) == nil {
			if output.Success {
				return &PublishResult{
					Platform:      "fanqie",
					Status:        "ok",
					PostID:        output.PostID,
					MaskedDisplay: maskedDisplay,
				}
			}
			return a.fail(classifyFanqieError(output.Error), output.Error, maskedDisplay)
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

// firstLine 取 text 的首行，限 maxRunes 字符。
func firstLine(text string, maxRunes int) string {
	lines := strings.SplitN(text, "\n", 2)
	if len(lines) == 0 {
		return "章节"
	}
	first := strings.TrimSpace(lines[0])
	if len([]rune(first)) <= maxRunes {
		return first
	}
	return "章节"
}

// classifyFanqieError 将脚本返回的错误信息映射为错误码。
func classifyFanqieError(errMsg string) string {
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

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
