package registry

import "L1_skills_register/models"

func (r *registryImpl) canAccess(pkg *models.SkillPackage, callerUID string) bool {
	switch pkg.Visibility {
	case "public":
		return true
	case "private":
		return pkg.OwnerUID == callerUID
	case "team":
		return false
	}
	return false
}

func (r *registryImpl) canModify(pkg *models.SkillPackage, callerUID string) bool {
	if pkg.Category == "preset" {
		return false
	}
	return pkg.OwnerUID == callerUID
}
