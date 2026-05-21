package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ============================================================
// 数据结构
// ============================================================

type Provider struct {
	APIKey  string   `json:"api_key"`
	BaseURL string   `json:"base_url"`
	Models  []string `json:"-"`
}

type ModelInfo struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Provider       string   `json:"provider"`
	ContextLimit   int      `json:"context_limit"`
	RecommendedFor string   `json:"recommended_for"`
	Tags           []string `json:"tags,omitempty"`
}

type Manager struct {
	config       ProviderConfig
	models       []ModelInfo
	rings        map[string]*KeyRing
	statePath    string
	loadWarnings []string
}

type ProviderConfig struct {
	Provider map[string]Provider `json:"provider"`
}

type KeysConfig map[string]KeysEntry

type KeysEntry struct {
	APIKeys []string `json:"api_keys"`
}

// ============================================================
// 内置厂商定义（base_url / 模型列表 写死在代码里，只从配置/环境变量取 API Key）
// ============================================================

type builtinDef struct {
	Key       string
	BaseURL   string
	EnvVars   []string
}

var builtinProviders = []builtinDef{
	{
		Key:     "deepseek",
		BaseURL: "https://api.deepseek.com/v1",
		EnvVars: []string{"DEEPSEEK_API_KEY", "TEAM_DEEPSEEK_API_KEY"},
	},
	{
		Key:     "hy3",
		BaseURL: "https://api.hy3-preview.tencent.com/v1",
		EnvVars: []string{"HY3_API_KEY", "TEAM_HY3_API_KEY"},
	},
}

// ============================================================
// 内置模型目录
// ============================================================

var builtinModels = []ModelInfo{
	{ID: "deepseek/deepseek-chat", Name: "DeepSeek V3 Chat", Provider: "deepseek", ContextLimit: 65536, RecommendedFor: "通用对话 · 日常写作 · 性价比高", Tags: []string{"fast", "cheap"}},
	{ID: "deepseek/deepseek-v4-flash", Name: "DeepSeek V4 Flash", Provider: "deepseek", ContextLimit: 65536, RecommendedFor: "快速响应 · 简单任务", Tags: []string{"flash", "fast"}},
	{ID: "deepseek/deepseek-v4-pro", Name: "DeepSeek V4 Pro", Provider: "deepseek", ContextLimit: 65536, RecommendedFor: "深度推理 · 复杂创作 · 逻辑性强", Tags: []string{"pro", "deep-thinking"}},
	{ID: "opencode/big-pickle", Name: "Big Pickle (免费)", Provider: "opencode", ContextLimit: 8192, RecommendedFor: "免费兜底 · 简单问答", Tags: []string{"free"}},
	{ID: "opencode/nemotron-3-super-free", Name: "Nemotron 3 (免费)", Provider: "opencode", ContextLimit: 8192, RecommendedFor: "免费兜底 · 简单问答", Tags: []string{"free"}},
	{ID: "hy3/hy3-preview", Name: "混元 3 预览", Provider: "hy3", ContextLimit: 32768, RecommendedFor: "免费兜底 · 通用场景 · 日常备用", Tags: []string{"free", "backup"}},
}

// ============================================================
// 核心逻辑
// ============================================================

func NewManager(keysConfigPath string) (*Manager, error) {
	m := &Manager{
		config: ProviderConfig{
			Provider: make(map[string]Provider),
		},
		rings: make(map[string]*KeyRing),
	}

	keysByProvider := m.loadKeys(keysConfigPath)

	for _, bp := range builtinProviders {
		keys := keysByProvider[bp.Key]
		if len(keys) > 0 {
			ring := NewKeyRing(bp.Key, keys, m.stateFilePath(keysConfigPath))
			m.rings[bp.Key] = ring

			m.config.Provider[bp.Key] = Provider{
				APIKey:  ring.Next(),
				BaseURL: bp.BaseURL,
				Models:  m.discoverModels(bp.Key),
			}
		}
	}

	if len(m.config.Provider) == 0 {
		return nil, fmt.Errorf(
			"no AI provider configured: create config/keys.json or set at least one API key env var (e.g. DEEPSEEK_API_KEY)",
		)
	}

	m.buildModelCatalog()
	return m, nil
}

func (m *Manager) stateFilePath(keysConfigPath string) string {
	if keysConfigPath != "" {
		dir := filepath.Dir(keysConfigPath)
		return filepath.Join(dir, ".keyring_state.json")
	}
	return "config/.keyring_state.json"
}

func (m *Manager) loadKeys(keysConfigPath string) map[string][]string {
	result := make(map[string][]string)
	usedFallback := make(map[string]bool)

	if keysConfigPath == "" {
		keysConfigPath = "config/keys.json"
	}

	cfg, err := loadKeysFile(keysConfigPath)
	fileExists := err == nil
	if fileExists && cfg != nil {
		for provID, entry := range *cfg {
			if len(entry.APIKeys) > 0 {
				result[provID] = entry.APIKeys
			}
		}
	}

	if !fileExists {
		m.loadWarnings = append(m.loadWarnings,
			fmt.Sprintf("keys config file not found: %s", keysConfigPath))
	}

	for _, bp := range builtinProviders {
		if len(result[bp.Key]) > 0 {
			continue
		}
		key := m.loadFromEnv(bp.EnvVars)
		if key != "" {
			result[bp.Key] = []string{key}
			usedFallback[bp.Key] = true
		}
	}

	if !fileExists && len(usedFallback) > 0 {
		for prov := range usedFallback {
			m.loadWarnings = append(m.loadWarnings,
				fmt.Sprintf("using env var fallback for provider %s (keys config file not found)", prov))
		}
	}

	return result
}

func loadKeysFile(path string) (*KeysConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg KeysConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse keys config %s: %w", path, err)
	}
	return &cfg, nil
}

func (m *Manager) loadFromEnv(envVars []string) string {
	for _, ev := range envVars {
		if v := os.Getenv(ev); v != "" {
			return v
		}
	}
	return ""
}

func (m *Manager) discoverModels(providerKey string) []string {
	var models []string
	for _, bm := range builtinModels {
		if bm.Provider == providerKey {
			models = append(models, strings.TrimPrefix(bm.ID, providerKey+"/"))
		}
	}
	if len(models) == 0 {
		models = []string{"default"}
	}
	return models
}

func (m *Manager) buildModelCatalog() {
	m.models = nil
	for _, bm := range builtinModels {
		if bm.Provider == "opencode" || bm.Provider == "hy3" {
			m.models = append(m.models, bm)
			continue
		}
		if _, ok := m.config.Provider[bm.Provider]; ok {
			m.models = append(m.models, bm)
		}
	}
}

func (m *Manager) ListModels() []ModelInfo {
	return m.models
}

func (m *Manager) HasModel(modelID string) bool {
	for _, mi := range m.models {
		if mi.ID == modelID {
			return true
		}
	}
	return false
}

func (m *Manager) GetProviderConfig() ProviderConfig {
	return m.config
}

// EnrichConfig 往沙箱配置中补充 provider 节。
// 每次调用会触发 KeyRing 轮转，切换到下一个 API Key（实现多 KEY 均等调用）。
func (m *Manager) EnrichConfig(configPath string) error {
	m.rotateKeys()

	if configPath == "" {
		return fmt.Errorf("configPath is required (sandbox config path, NOT ~/.config/opencode/opencode.json)")
	}

	var existing map[string]interface{}
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("parse existing config %s: %w", configPath, err)
		}
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}

	delete(existing, "$schema")
	existing["provider"] = m.config.Provider

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal enriched config: %w", err)
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir %s: %w", dir, err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("write enriched config: %w", err)
	}

	if err := os.Chmod(configPath, 0600); err != nil {
		return fmt.Errorf("chmod config: %w", err)
	}

	return nil
}

func (m *Manager) rotateKeys() {
	for key, ring := range m.rings {
		if prov, ok := m.config.Provider[key]; ok {
			prov.APIKey = ring.Next()
			m.config.Provider[key] = prov
		}
	}
}

func OpenCodeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

func (m *Manager) Validate() []string {
	var warnings []string
	warnings = append(warnings, m.loadWarnings...)
	if len(m.config.Provider) == 0 {
		warnings = append(warnings, "no providers configured (no API keys found in keyring or environment)")
	}
	for k, p := range m.config.Provider {
		if p.APIKey == "" {
			warnings = append(warnings, fmt.Sprintf("provider %s has empty API key", k))
		}
		if p.BaseURL == "" {
			warnings = append(warnings, fmt.Sprintf("provider %s has empty base URL", k))
		}
		if len(p.Models) == 0 {
			warnings = append(warnings, fmt.Sprintf("provider %s has no models", k))
		}
	}
	return warnings
}
