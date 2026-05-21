package a4md

import "strings"

// escapeForMD 对注入 MD 的动态内容进行转义
// 调用时机：在组装 TemplateData 时，对所有来自用户/LLM/DB 的字符串字段调用
//
// 不转义的内容（已存在于模板中）：
//   - 标题字符 # ## ###
//   - 表格管道线 | |
//   - 模板内的固定文本
//
// 必须转义的内容：
//   - Topic、UserIntent、Decisions
//   - Products 所有字段
//   - PostID、PostURL、ErrorCode、ErrorMsg
//   - Stats 中的 AccountID
func escapeForMD(s string) string {
	if s == "" {
		return s
	}
	replacer := strings.NewReplacer(
		`\`, `\\`,
		"`", "\\`",
		"*", "\\*",
		"_", "\\_",
		"{", "\\{",
		"}", "\\}",
		"[", "\\[",
		"]", "\\]",
		"<", "\\<",
		">", "\\>",
		"(", "\\(",
		")", "\\)",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		".", "\\.",
		"!", "\\!",
		"|", "\\|",
		"~", "\\~",
	)
	return replacer.Replace(s)
}

func escapeProductMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	escaped := make(map[string]string, len(m))
	for k, v := range m {
		escaped[k] = escapeForMD(v)
	}
	return escaped
}
