package registry

import (
	"context"
	"fmt"

	"L1_skills_register/models"
	"L1_skills_register/store"
)

func (r *registryImpl) AllocSkill(ctx context.Context, platform, theme, style string) (*models.AllocSkillResponse, error) {
	if platform == "" {
		return nil, fmt.Errorf("platform is required")
	}

	result, err := r.store.AllocSkill(ctx, platform, theme, style)
	if err != nil {
		if err == store.ErrNoAvailableSkill {
			return nil, err
		}
		return nil, fmt.Errorf("alloc skill failed: %w", err)
	}

	return result, nil
}
