package store

import (
	"context"

	"L1_skills_register/models"
)

type SkillStore interface {
	Save(ctx context.Context, pkg *models.SkillPackage, promptContent []byte) error
	Get(ctx context.Context, skillID, version string) (*models.SkillPackage, error)
	GetLatest(ctx context.Context, skillID string) (*models.SkillPackage, error)
	List(ctx context.Context, filter models.SkillFilter) ([]models.SkillSummary, error)
	Deprecate(ctx context.Context, skillID, version string) error
	Exists(ctx context.Context, skillID, version string) (bool, error)
	CountByOwner(ctx context.Context, ownerUID string) (int, error)
}
