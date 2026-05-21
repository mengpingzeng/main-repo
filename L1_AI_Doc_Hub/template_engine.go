package a4md

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/md/*.tmpl
var templateFS embed.FS

// TemplateEngine 模板渲染引擎
type TemplateEngine struct {
	tmpl     *template.Template
	tmplName string
	version  string
}

// NewTemplateEngine 创建渲染引擎
// version 参数指定模板版本（如 "v1"），通过 embed.FS 加载对应文件
func NewTemplateEngine(version string) (*TemplateEngine, error) {
	funcs := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}

	patterns := []string{
		fmt.Sprintf("templates/md/task_%s.md.tmpl", version),
		fmt.Sprintf("templates/md/stats_section_%s.md.tmpl", version),
	}

	tmpl, err := template.New("a4md").Funcs(funcs).ParseFS(templateFS, patterns...)
	if err != nil {
		return nil, fmt.Errorf("a4md: failed to parse templates: %w", err)
	}

	return &TemplateEngine{
		tmpl:     tmpl,
		tmplName: fmt.Sprintf("task_%s.md.tmpl", version),
		version:  version,
	}, nil
}

// Render 执行主模板渲染，产出最终 MD 字符串
func (e *TemplateEngine) Render(data *TemplateData) (string, error) {
	var buf bytes.Buffer
	if err := e.tmpl.ExecuteTemplate(&buf, e.tmplName, data); err != nil {
		return "", fmt.Errorf("a4md: template render failed: %w", err)
	}
	return buf.String(), nil
}

// RenderStatsSection 渲染追加段落的模板
// 从 embed.FS 加载 stats_section_{version}.md.tmpl
func (e *TemplateEngine) RenderStatsSection(period string, stats []StatItem) (string, error) {
	name := fmt.Sprintf("stats_section_%s.md.tmpl", e.version)

	data := struct {
		Period string
		Items  []StatItem
	}{Period: period, Items: stats}

	var buf bytes.Buffer
	if err := e.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("a4md: render stats section failed: %w", err)
	}
	return buf.String(), nil
}
