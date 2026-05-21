package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"L1_skills_register/models"
)

type FakeSkillStore struct {
	mu     sync.RWMutex
	skills map[string]*skillEntry
}

type skillEntry struct {
	Pkg           *models.SkillPackage
	PromptContent []byte
}

func NewFakeSkillStore() *FakeSkillStore {
	return &FakeSkillStore{
		skills: make(map[string]*skillEntry),
	}
}

func key(skillID, version string) string {
	return skillID + ":" + version
}

func (s *FakeSkillStore) Save(ctx context.Context, pkg *models.SkillPackage, promptContent []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := key(pkg.ID, pkg.Version)
	if _, exists := s.skills[k]; exists {
		return fmt.Errorf("version_conflict: skill %s version %s already exists", pkg.ID, pkg.Version)
	}

	pkgCopy := *pkg
	pcCopy := make([]byte, len(promptContent))
	copy(pcCopy, promptContent)
	pkgCopy.PromptContent = string(pcCopy)

	s.skills[k] = &skillEntry{
		Pkg:           &pkgCopy,
		PromptContent: pcCopy,
	}
	return nil
}

func (s *FakeSkillStore) Get(ctx context.Context, skillID, version string) (*models.SkillPackage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if version != "" {
		k := key(skillID, version)
		entry, ok := s.skills[k]
		if !ok {
			return nil, fmt.Errorf("skill not found: %s:%s", skillID, version)
		}
		pkgCopy := *entry.Pkg
		return &pkgCopy, nil
	}

	return s.GetLatest(ctx, skillID)
}

func (s *FakeSkillStore) GetLatest(ctx context.Context, skillID string) (*models.SkillPackage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var latest *models.SkillPackage
	var latestVer string

	for k, entry := range s.skills {
		if strings.HasPrefix(k, skillID+":") && entry.Pkg.Status == "active" {
			if latest == nil || compareVersions(entry.Pkg.Version, latestVer) > 0 {
				latest = entry.Pkg
				latestVer = entry.Pkg.Version
			}
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("skill not found: %s", skillID)
	}

	pkgCopy := *latest
	return &pkgCopy, nil
}

func (s *FakeSkillStore) List(ctx context.Context, filter models.SkillFilter) ([]models.SkillSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]models.SkillSummary, 0)
	for _, entry := range s.skills {
		pkg := entry.Pkg

		if filter.Category != "" && pkg.Category != filter.Category {
			continue
		}
		if filter.Visibility != "" && pkg.Visibility != filter.Visibility {
			continue
		}
		if filter.Status != "" && pkg.Status != filter.Status {
			continue
		}
		if filter.OwnerUID != "" && pkg.OwnerUID != filter.OwnerUID {
			continue
		}
		if filter.Search != "" {
			searchLower := strings.ToLower(filter.Search)
			if !strings.Contains(strings.ToLower(pkg.Name), searchLower) &&
				!strings.Contains(strings.ToLower(pkg.Description), searchLower) {
				continue
			}
		}

		result = append(result, models.SkillSummary{
			SkillID:          pkg.ID,
			Version:          pkg.Version,
			Name:             pkg.Name,
			Description:      pkg.Description,
			Category:         pkg.Category,
			ModelRecommended: pkg.ModelRecommended,
			Visibility:       pkg.Visibility,
			Status:           pkg.Status,
			ScriptsPath:      pkg.ScriptsPath,
			TemplatesPath:    pkg.TemplatesPath,
			ExamplesPath:     pkg.ExamplesPath,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].SkillID < result[j].SkillID
	})

	if filter.Offset > 0 && filter.Offset < len(result) {
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result, nil
}

func (s *FakeSkillStore) Deprecate(ctx context.Context, skillID, version string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := key(skillID, version)
	entry, ok := s.skills[k]
	if !ok {
		return fmt.Errorf("skill not found: %s:%s", skillID, version)
	}

	entry.Pkg.Status = "deprecated"
	return nil
}

func (s *FakeSkillStore) Exists(ctx context.Context, skillID, version string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.skills[key(skillID, version)]
	return ok, nil
}

func (s *FakeSkillStore) CountByOwner(ctx context.Context, ownerUID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, entry := range s.skills {
		if entry.Pkg.OwnerUID == ownerUID && entry.Pkg.Category == "custom" {
			count++
		}
	}
	return count, nil
}

func compareVersions(a, b string) int {
	partsA := parseVersion(a)
	partsB := parseVersion(b)

	for i := 0; i < 3; i++ {
		if partsA[i] > partsB[i] {
			return 1
		}
		if partsA[i] < partsB[i] {
			return -1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	var parts [3]int
	fmt.Sscanf(v, "%d.%d.%d", &parts[0], &parts[1], &parts[2])
	return parts
}
