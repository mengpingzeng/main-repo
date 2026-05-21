package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type PostHook func(rawOutput string, ctx HookContext) (finalOutput string, err error)
type PreHook func(rawInput string, ctx HookContext) (finalInput string, err error)

type HookContext struct {
	Ctx     context.Context
	SkillID string
	Topic   string
}

var OfficialPostHooks = map[string]PostHook{
	"xhs_format":          formatXhsPostLayout,
	"wechat_md_to_html":   convertMarkdownToWechatHTML,
	"sensitive_filter":    filterSensitiveWords,
	"append_default_tags": appendDefaultTags,
}

var OfficialPreHooks = map[string]PreHook{
	"sensitive_filter_input": filterSensitiveInput,
}

var OfficialPostHookNames = func() []string {
	names := make([]string, 0, len(OfficialPostHooks))
	for name := range OfficialPostHooks {
		names = append(names, name)
	}
	return names
}()

var OfficialPreHookNames = func() []string {
	names := make([]string, 0, len(OfficialPreHooks))
	for name := range OfficialPreHooks {
		names = append(names, name)
	}
	return names
}()

func IsOfficialPostHook(name string) bool {
	_, ok := OfficialPostHooks[name]
	return ok
}

func IsOfficialPreHook(name string) bool {
	_, ok := OfficialPreHooks[name]
	return ok
}

func ExecutePostHook(name, rawOutput string, hookCtx HookContext) (string, error) {
	hook, ok := OfficialPostHooks[name]
	if !ok {
		return rawOutput, fmt.Errorf("unknown post_hook: %s", name)
	}

	ctx, cancel := context.WithTimeout(hookCtx.Ctx, 500*time.Millisecond)
	defer cancel()
	hookCtx.Ctx = ctx

	return hook(rawOutput, hookCtx)
}

func ExecutePreHook(name, rawInput string, hookCtx HookContext) (string, error) {
	hook, ok := OfficialPreHooks[name]
	if !ok {
		return rawInput, fmt.Errorf("unknown pre_hook: %s", name)
	}

	ctx, cancel := context.WithTimeout(hookCtx.Ctx, 500*time.Millisecond)
	defer cancel()
	hookCtx.Ctx = ctx

	return hook(rawInput, hookCtx)
}

var sensitiveWords = []string{
	"赌博", "毒品", "枪支",
}

func filterSensitiveWords(rawOutput string, ctx HookContext) (string, error) {
	result := rawOutput
	for _, word := range sensitiveWords {
		result = strings.ReplaceAll(result, word, "***")
	}
	return result, nil
}

func filterSensitiveInput(rawInput string, ctx HookContext) (string, error) {
	result := rawInput
	for _, word := range sensitiveWords {
		result = strings.ReplaceAll(result, word, "***")
	}
	return result, nil
}

func formatXhsPostLayout(rawOutput string, ctx HookContext) (string, error) {
	lines := strings.Split(rawOutput, "\n")
	result := make([]string, 0, len(lines))
	var tags []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			tags = append(tags, trimmed)
			continue
		}
		if trimmed == "" && len(result) > 0 && result[len(result)-1] == "" {
			continue
		}
		result = append(result, trimmed)
	}

	for len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}

	if len(tags) > 0 {
		result = append(result, "")
		result = append(result, tags...)
	}

	return strings.Join(result, "\n"), nil
}

func convertMarkdownToWechatHTML(rawOutput string, ctx HookContext) (string, error) {
	boldRe := regexp.MustCompile(`\*\*(.+?)\*\*`)
	html := boldRe.ReplaceAllString(rawOutput, `<strong>$1</strong>`)

	italicRe := regexp.MustCompile(`\*(.+?)\*`)
	html = italicRe.ReplaceAllString(html, `<em>$1</em>`)

	parts := strings.Split(html, "\n\n")
	sections := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			sections = append(sections, fmt.Sprintf(`<h3>%s</h3>`, trimmed[4:]))
		} else if strings.HasPrefix(trimmed, "## ") {
			sections = append(sections, fmt.Sprintf(`<h2>%s</h2>`, trimmed[3:]))
		} else if strings.HasPrefix(trimmed, "# ") {
			sections = append(sections, fmt.Sprintf(`<h1>%s</h1>`, trimmed[2:]))
		} else {
			lines := strings.Split(trimmed, "\n")
			para := strings.Join(lines, "<br>")
			sections = append(sections, fmt.Sprintf(`<section><p>%s</p></section>`, para))
		}
	}

	return fmt.Sprintf(`<div class="wechat-article">%s</div>`, strings.Join(sections, "\n")), nil
}

func appendDefaultTags(rawOutput string, ctx HookContext) (string, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(rawOutput), &data); err != nil {
		return rawOutput, nil
	}

	tags, ok := data["tags"].([]interface{})
	if !ok || len(tags) == 0 {
		tags = []interface{}{}
	}

	defaultTags := getDefaultTags(ctx.Topic)
	for _, dt := range defaultTags {
		found := false
		for _, t := range tags {
			if s, ok := t.(string); ok && s == dt {
				found = true
				break
			}
		}
		if !found {
			tags = append(tags, dt)
		}
	}

	data["tags"] = tags
	result, err := json.Marshal(data)
	if err != nil {
		return rawOutput, nil
	}
	return string(result), nil
}

func getDefaultTags(topic string) []string {
	return []string{fmt.Sprintf("#%s", topic), "#种草", "#分享"}
}
