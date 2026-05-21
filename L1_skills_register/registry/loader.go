package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"L1_skills_register/models"
)

type LoadResult struct {
	Dir        string `json:"dir"`
	Registered bool   `json:"registered"`
	Skipped    bool   `json:"skipped"`
	Error      string `json:"error,omitempty"`
}

type LoadSummary struct {
	Total      int          `json:"total"`
	Registered int          `json:"registered"`
	Skipped    int          `json:"skipped"`
	Errors     int          `json:"errors"`
	Results    []LoadResult `json:"results"`
}

func (r *registryImpl) LoadFromDirectory(ctx context.Context, dirPath string) (*LoadSummary, error) {
	summary := &LoadSummary{Results: make([]LoadResult, 0)}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dirPath, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillDir := filepath.Join(dirPath, entry.Name())
		metaPath := filepath.Join(skillDir, "_meta.json")

		if _, err := os.Stat(metaPath); os.IsNotExist(err) {
			continue
		}

		summary.Total++

		result, err := r.loadOneSkill(ctx, skillDir, entry.Name())
		if err != nil {
			result.Error = err.Error()
			summary.Errors++
		} else if result.Skipped {
			summary.Skipped++
		} else {
			summary.Registered++
		}
		summary.Results = append(summary.Results, *result)
	}

	return summary, nil
}

type skillMeta struct {
	OwnerID     string `json:"ownerId"`
	Slug        string `json:"slug"`
	Version     string `json:"version"`
	PublishedAt int64  `json:"publishedAt"`
}

func (r *registryImpl) loadOneSkill(ctx context.Context, skillDir, dirName string) (*LoadResult, error) {
	result := &LoadResult{Dir: dirName}

	metaPath := filepath.Join(skillDir, "_meta.json")
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return result, fmt.Errorf("read _meta.json: %w", err)
	}

	var meta skillMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return result, fmt.Errorf("parse _meta.json: %w", err)
	}

	if meta.Slug == "" {
		return result, fmt.Errorf("_meta.json missing slug")
	}
	if meta.Version == "" {
		return result, fmt.Errorf("_meta.json missing version")
	}

	skillID := meta.Slug
	exists, err := r.store.Exists(ctx, skillID, meta.Version)
	if err != nil {
		return result, fmt.Errorf("check exists: %w", err)
	}
	if exists {
		result.Skipped = true
		return result, nil
	}

	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	promptBytes, err := os.ReadFile(skillMDPath)
	if err != nil {
		return result, fmt.Errorf("read SKILL.md: %w", err)
	}

	name := extractName(promptBytes)
	description := extractDescription(promptBytes)
	if name == "" {
		name = dirName
	}

	scriptsPath := ""
	if _, err := os.Stat(filepath.Join(skillDir, "scripts")); err == nil {
		scriptsPath = "scripts/"
	}
	templatesPath := ""
	if _, err := os.Stat(filepath.Join(skillDir, "templates")); err == nil {
		templatesPath = "templates/"
	}
	examplesPath := ""
	if _, err := os.Stat(filepath.Join(skillDir, "examples")); err == nil {
		examplesPath = "examples/"
	}

	pkg := &models.SkillPackage{
		ID:               skillID,
		Name:             name,
		Version:          meta.Version,
		Category:         "custom",
		Description:      description,
		PromptContent:    string(promptBytes),
		OutputSchema:     map[string]interface{}{"type": "object"},
		Visibility:       "public",
		Status:           "active",
		OwnerUID:         meta.OwnerID,
		ScriptsPath:      scriptsPath,
		TemplatesPath:    templatesPath,
		ExamplesPath:     examplesPath,
		SkillDirectory:   skillDir,
	}

	if err := r.store.Save(ctx, pkg, promptBytes); err != nil {
		return result, fmt.Errorf("save: %w", err)
	}

	result.Registered = true
	return result, nil
}

func extractName(md []byte) string {
	content := string(md)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			name := strings.TrimPrefix(line, "# ")
			name = strings.TrimSuffix(name, "\r")
			return strings.TrimSpace(name)
		}
	}
	return ""
}

func extractDescription(md []byte) string {
	content := string(md)
	lines := strings.Split(content, "\n")
	inContent := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			if inContent {
				return ""
			}
			inContent = true
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "**") && strings.Contains(trimmed, "**") {
			clean := strings.Trim(trimmed, "*")
			clean = strings.TrimSpace(clean)
			if strings.Contains(clean, "描述") || strings.Contains(clean, "：") {
				parts := strings.SplitN(clean, "：", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
				parts = strings.SplitN(clean, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
		if inContent && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "**") {
			return trimmed
		}
	}
	return ""
}
