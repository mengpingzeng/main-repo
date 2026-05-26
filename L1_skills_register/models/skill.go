package models

type SkillPackage struct {
	ID              string                 `yaml:"id" json:"skill_id"`
	Name            string                 `yaml:"name" json:"name"`
	Version         string                 `yaml:"version" json:"version"`
	Category        string                 `yaml:"category" json:"category"`
	Description     string                 `yaml:"description" json:"description"`
	Inputs          map[string]InputDef    `yaml:"inputs" json:"inputs,omitempty"`
	Targets         []string               `yaml:"targets" json:"targets"`
	ModelRecommended *ModelRecommended     `yaml:"model_recommended" json:"model_recommended"`
	PromptTemplate  string                 `yaml:"prompt_template" json:"prompt_template"`
	PromptContent   string                 `yaml:"-" json:"-"`
	OutputSchema    map[string]interface{} `yaml:"output_schema" json:"output_schema"`
	PreHook         string                 `yaml:"pre_hook" json:"pre_hook,omitempty"`
	PostHook        string                 `yaml:"post_hook" json:"post_hook,omitempty"`
	Visibility      string                 `yaml:"visibility" json:"visibility"`
	Status          string                 `yaml:"status" json:"status"`
	OwnerUID        string                 `yaml:"owner_uid" json:"owner_uid,omitempty"`
	ScriptsPath     string                 `yaml:"scripts_path" json:"scripts_path,omitempty"`
	TemplatesPath   string                 `yaml:"templates_path" json:"templates_path,omitempty"`
	ExamplesPath    string                 `yaml:"examples_path" json:"examples_path,omitempty"`
	SkillDirectory  string                 `yaml:"-" json:"-"`
}

type InputDef struct {
	Type     string `yaml:"type" json:"type"`
	Required bool   `yaml:"required" json:"required"`
	Default  string `yaml:"default,omitempty" json:"default,omitempty"`
}

type ModelRecommended struct {
	Primary  string   `yaml:"primary" json:"primary"`
	Fallback []string `yaml:"fallback" json:"fallback"`
}

type SkillSummary struct {
	SkillID          string            `json:"skill_id"`
	Version          string            `json:"version"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Category         string            `json:"category"`
	ModelRecommended *ModelRecommended `json:"model_recommended"`
	Visibility       string            `json:"visibility"`
	Status           string            `json:"status"`
	ScriptsPath      string            `json:"scripts_path,omitempty"`
	TemplatesPath    string            `json:"templates_path,omitempty"`
	ExamplesPath     string            `json:"examples_path,omitempty"`
}

type SkillFilter struct {
	Category   string
	Visibility string
	Status     string
	OwnerUID   string
	Search     string
	Limit      int
	Offset     int
}

type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

type BootstrapRequest struct {
	Skills []BootstrapSkill `json:"skills"`
}

type BootstrapSkill struct {
	SkillYAML     string `json:"skill_yaml"`
	PromptContent string `json:"prompt_content"`
}

type BootstrapResponse struct {
	Registered int      `json:"registered"`
	Skipped    int      `json:"skipped"`
	Errors     []string `json:"errors"`
}

type RegisterRequest struct {
	SkillYAML     string `json:"skill_yaml"`
	PromptContent string `json:"prompt_content"`
	OwnerUID      string `json:"owner_uid"`
}

type DeprecateRequest struct {
	Version string `json:"version"`
}

type AllocSkillRequest struct {
	Platform string `json:"platform"`
	Theme    string `json:"theme,omitempty"`
	Style    string `json:"style,omitempty"`
}

type AllocSkillResponse struct {
	SkillID          string            `json:"skill_id"`
	Version          string            `json:"version"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Category         string            `json:"category"`
	ModelRecommended *ModelRecommended `json:"model_recommended,omitempty"`
}

type ApiError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
