package registry

import (
	"encoding/json"
	"fmt"
	"regexp"

	"L1_skills_register/hooks"
	"L1_skills_register/models"

	"gopkg.in/yaml.v3"
)

var semverRx = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func (r *registryImpl) Validate(skillYAML []byte) (*models.ValidationResult, error) {
	result := &models.ValidationResult{Valid: true, Errors: []string{}}

	var pkg models.SkillPackage
	if err := yaml.Unmarshal(skillYAML, &pkg); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("YAML syntax error: %v", err))
		return result, nil
	}

	addErr := func(msg string) {
		result.Valid = false
		result.Errors = append(result.Errors, msg)
	}

	if pkg.ID == "" {
		addErr("skill_id is required")
	}
	if pkg.Name == "" {
		addErr("name is required")
	}
	if pkg.Version == "" {
		addErr("version is required")
	} else if !isValidSemVer(pkg.Version) {
		addErr("version must follow semver: X.Y.Z (e.g., 1.0.0)")
	}
	if pkg.Category == "" {
		addErr("category is required (preset|team|custom)")
	} else if !isValidCategory(pkg.Category) {
		addErr("category must be one of: preset, team, custom")
	}
	if pkg.PromptTemplate == "" {
		addErr("prompt_template is required")
	}
	if pkg.OutputSchema == nil || len(pkg.OutputSchema) == 0 {
		addErr("output_schema is required and must not be empty")
	} else {
		if err := validateJSONSchemaStructure(pkg.OutputSchema); err != nil {
			addErr(fmt.Sprintf("output_schema invalid: %v", err))
		}
	}

	if pkg.PostHook != "" {
		if !hooks.IsOfficialPostHook(pkg.PostHook) {
			addErr(fmt.Sprintf("unknown post_hook: %s, must be one of: %v",
				pkg.PostHook, hooks.OfficialPostHookNames))
		}
	}
	if pkg.PreHook != "" {
		if !hooks.IsOfficialPreHook(pkg.PreHook) {
			addErr(fmt.Sprintf("unknown pre_hook: %s, must be one of: %v",
				pkg.PreHook, hooks.OfficialPreHookNames))
		}
	}

	if pkg.Visibility != "" && !isValidVisibility(pkg.Visibility) {
		addErr("visibility must be one of: public, private, team")
	}

	return result, nil
}

func isValidSemVer(v string) bool {
	return semverRx.MatchString(v)
}

func isValidCategory(c string) bool {
	switch c {
	case "preset", "team", "custom":
		return true
	}
	return false
}

func isValidVisibility(v string) bool {
	switch v {
	case "public", "private", "team":
		return true
	}
	return false
}

func validateJSONSchemaStructure(schema map[string]interface{}) error {
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("cannot marshal schema: %v", err)
	}

	var js map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &js); err != nil {
		return fmt.Errorf("invalid JSON structure: %v", err)
	}

	typeVal, hasType := js["type"]
	if !hasType {
		return fmt.Errorf("schema must have a 'type' field")
	}
	if t, ok := typeVal.(string); !ok || t != "object" {
		return fmt.Errorf("root schema type must be 'object'")
	}

	return nil
}
