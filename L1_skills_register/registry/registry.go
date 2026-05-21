package registry

import (
	"context"
	"encoding/base64"
	"fmt"

	"L1_skills_register/models"

	"gopkg.in/yaml.v3"
)

func (r *registryImpl) Register(ctx context.Context, skillYAML []byte, promptContent []byte, ownerUID string) (*models.SkillPackage, error) {
	if len(skillYAML) == 0 {
		return nil, fmt.Errorf("skill_yaml is empty after decoding")
	}
	result, err := r.Validate(skillYAML)
	if err != nil {
		return nil, fmt.Errorf("validate error: %w", err)
	}
	if !result.Valid {
		return nil, fmt.Errorf("validation failed: %v", result.Errors)
	}

	var pkg models.SkillPackage
	if err := yaml.Unmarshal(skillYAML, &pkg); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}

	pkg.PromptContent = string(promptContent)
	if pkg.Status == "" {
		pkg.Status = "active"
	}
	if pkg.Category == "" {
		pkg.Category = "custom"
	}
	if pkg.Visibility == "" {
		pkg.Visibility = "private"
	}
	if ownerUID != "" {
		pkg.OwnerUID = ownerUID
	}

	if pkg.Category == "custom" {
		if err := r.checkQuota(ctx, pkg.OwnerUID); err != nil {
			return nil, err
		}
	}

	exists, err := r.store.Exists(ctx, pkg.ID, pkg.Version)
	if err != nil {
		return nil, fmt.Errorf("check exists: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("version_conflict: skill %s version %s already exists", pkg.ID, pkg.Version)
	}

	if err := r.store.Save(ctx, &pkg, promptContent); err != nil {
		return nil, fmt.Errorf("save: %w", err)
	}

	pkgCopy := pkg
	return &pkgCopy, nil
}

func (r *registryImpl) Bootstrap(ctx context.Context, skills []models.BootstrapSkill) (*models.BootstrapResponse, error) {
	resp := &models.BootstrapResponse{}

	for i, bs := range skills {
		yamlBytes, err := base64.StdEncoding.DecodeString(bs.SkillYAML)
		if err != nil {
			resp.Errors = append(resp.Errors, fmt.Sprintf("skill[%d] base64 decode yaml: %v", i, err))
			continue
		}
		promptBytes, err := base64.StdEncoding.DecodeString(bs.PromptContent)
		if err != nil {
			resp.Errors = append(resp.Errors, fmt.Sprintf("skill[%d] base64 decode prompt: %v", i, err))
			continue
		}

		pkg, err := r.Register(ctx, yamlBytes, promptBytes, "")
		if err != nil {
			if contains(err.Error(), "already exists") {
				resp.Skipped++
			} else {
				resp.Errors = append(resp.Errors, fmt.Sprintf("skill[%d] %s", i, err.Error()))
			}
			continue
		}
		_ = pkg
		resp.Registered++
	}

	return resp, nil
}

func (r *registryImpl) Get(ctx context.Context, skillID, version string) (*models.SkillPackage, error) {
	return r.store.Get(ctx, skillID, version)
}

func (r *registryImpl) List(ctx context.Context, filter models.SkillFilter) ([]models.SkillSummary, error) {
	if filter.Status == "" {
		filter.Status = "active"
	}
	return r.store.List(ctx, filter)
}

func (r *registryImpl) Deprecate(ctx context.Context, skillID, version string) error {
	return r.store.Deprecate(ctx, skillID, version)
}

func (r *registryImpl) checkQuota(ctx context.Context, ownerUID string) error {
	count, err := r.store.CountByOwner(ctx, ownerUID)
	if err != nil {
		return err
	}
	if count >= r.maxCustomPerUser {
		return fmt.Errorf("quota exceeded: max %d custom skills per user", r.maxCustomPerUser)
	}
	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
