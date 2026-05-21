package registry

import (
	"context"

	"L1_skills_register/models"
	"L1_skills_register/store"
)

type Registry interface {
	Register(ctx context.Context, skillYAML []byte, promptContent []byte, ownerUID string) (*models.SkillPackage, error)
	Bootstrap(ctx context.Context, skills []models.BootstrapSkill) (*models.BootstrapResponse, error)
	Get(ctx context.Context, skillID, version string) (*models.SkillPackage, error)
	List(ctx context.Context, filter models.SkillFilter) ([]models.SkillSummary, error)
	Deprecate(ctx context.Context, skillID, version string) error
	Validate(skillYAML []byte) (*models.ValidationResult, error)
	LoadFromDirectory(ctx context.Context, dirPath string) (*LoadSummary, error)
}

type registryImpl struct {
	store store.SkillStore
	maxCustomPerUser int
}

func New(store store.SkillStore) Registry {
	return &registryImpl{
		store:            store,
		maxCustomPerUser: 20,
	}
}

func NewWithConfig(store store.SkillStore, maxCustomPerUser int) Registry {
	if maxCustomPerUser <= 0 {
		maxCustomPerUser = 20
	}
	return &registryImpl{
		store:            store,
		maxCustomPerUser: maxCustomPerUser,
	}
}
