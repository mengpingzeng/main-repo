package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"L1_skills_register/models"
	"L1_skills_register/registry"
)

const InternalAuthHeader = "X-Internal-Token"

type Handler struct {
	reg          registry.Registry
	internalAuth string
}

func NewHandler(reg registry.Registry, internalAuth string) *Handler {
	return &Handler{
		reg:          reg,
		internalAuth: internalAuth,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/skill/status", h.handleStatus)
	mux.HandleFunc("/api/skill/list", h.handleList)
	mux.HandleFunc("/api/skill/register", h.handleRegister)
	mux.HandleFunc("/api/skill/validate", h.handleValidate)
	mux.HandleFunc("/api/skill/bootstrap", h.handleBootstrap)
	mux.HandleFunc("/api/skill/", h.handleSkillByID)
}

func (h *Handler) requireInternalAuth(r *http.Request) bool {
	if h.internalAuth == "" {
		return true
	}
	token := r.Header.Get(InternalAuthHeader)
	return token == h.internalAuth
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"version": "1.0.0",
		"service": "skill-registry",
	})
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET allowed")
		return
	}

	filter := models.SkillFilter{
		Category:   r.URL.Query().Get("category"),
		Visibility: r.URL.Query().Get("visibility"),
		OwnerUID:   r.URL.Query().Get("owner_uid"),
		Search:     r.URL.Query().Get("search"),
		Status:     "active",
	}

	skills, err := h.reg.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	if skills == nil {
		skills = []models.SkillSummary{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"skills": skills,
		"total":  len(skills),
	})
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST allowed")
		return
	}

	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON: "+err.Error())
		return
	}

	if req.SkillYAML == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "skill_yaml is required")
		return
	}
	if req.PromptContent == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "prompt_content is required")
		return
	}

	yamlBytes, err := base64.StdEncoding.DecodeString(req.SkillYAML)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "skill_yaml must be base64 encoded")
		return
	}
	promptBytes, err := base64.StdEncoding.DecodeString(req.PromptContent)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "prompt_content must be base64 encoded")
		return
	}

	pkg, err := h.reg.Register(r.Context(), yamlBytes, promptBytes, req.OwnerUID)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "version_conflict") {
			code = http.StatusConflict
		} else if strings.Contains(err.Error(), "validation failed") {
			code = http.StatusBadRequest
		} else if strings.Contains(err.Error(), "quota exceeded") {
			code = http.StatusTooManyRequests
		}
		writeError(w, code, "register_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"skill_id": pkg.ID,
		"version":  pkg.Version,
		"status":   pkg.Status,
	})
}

func (h *Handler) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST allowed")
		return
	}

	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON: "+err.Error())
		return
	}

	yamlBase64, ok := req["skill_yaml"]
	if !ok || yamlBase64 == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "skill_yaml is required")
		return
	}

	yamlBytes, err := base64.StdEncoding.DecodeString(yamlBase64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "skill_yaml must be base64 encoded")
		return
	}

	result, err := h.reg.Validate(yamlBytes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "validate_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST allowed")
		return
	}

	if !h.requireInternalAuth(r) {
		writeError(w, http.StatusForbidden, "forbidden", "internal auth required")
		return
	}

	var req models.BootstrapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON: "+err.Error())
		return
	}

	resp, err := h.reg.Bootstrap(r.Context(), req.Skills)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "bootstrap_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleSkillByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/skill/")

	if path == "" {
		writeError(w, http.StatusNotFound, "not_found", "skill id required")
		return
	}

	parts := strings.SplitN(path, "/", 2)
	skillID := parts[0]

	if len(parts) == 2 && parts[1] == "deprecate" {
		h.handleDeprecate(w, r, skillID)
		return
	}

	if !h.requireInternalAuth(r) {
		dr := struct {
			SkillID          string            `json:"skill_id"`
			Version          string            `json:"version"`
			Name             string            `json:"name"`
			Description      string            `json:"description"`
			Category         string            `json:"category"`
			ModelRecommended *models.ModelRecommended `json:"model_recommended"`
			Visibility       string            `json:"visibility"`
			Status           string            `json:"status"`
			ScriptsPath      string            `json:"scripts_path,omitempty"`
			TemplatesPath    string            `json:"templates_path,omitempty"`
			ExamplesPath     string            `json:"examples_path,omitempty"`
		}{
			SkillID: skillID,
		}

		version := r.URL.Query().Get("version")
		pkg, err := h.reg.Get(r.Context(), skillID, version)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "skill not found: "+skillID)
			return
		}

		dr.Version = pkg.Version
		dr.Name = pkg.Name
		dr.Description = pkg.Description
		dr.Category = pkg.Category
		dr.ModelRecommended = pkg.ModelRecommended
		dr.Visibility = pkg.Visibility
		dr.Status = pkg.Status
		dr.ScriptsPath = pkg.ScriptsPath
		dr.TemplatesPath = pkg.TemplatesPath
		dr.ExamplesPath = pkg.ExamplesPath

		writeJSON(w, http.StatusOK, dr)
		return
	}

	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET allowed")
		return
	}

	version := r.URL.Query().Get("version")

	pkg, err := h.reg.Get(r.Context(), skillID, version)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "skill not found: "+skillID)
		return
	}

	fullResp := map[string]interface{}{
		"skill_id":           pkg.ID,
		"version":            pkg.Version,
		"name":               pkg.Name,
		"description":        pkg.Description,
		"category":           pkg.Category,
		"model_recommended":  pkg.ModelRecommended,
		"prompt_content":     pkg.PromptContent,
		"output_schema":      pkg.OutputSchema,
		"pre_hook":           pkg.PreHook,
		"post_hook":          pkg.PostHook,
		"visibility":         pkg.Visibility,
		"status":             pkg.Status,
		"owner_uid":          pkg.OwnerUID,
		"scripts_path":       pkg.ScriptsPath,
		"templates_path":     pkg.TemplatesPath,
		"examples_path":      pkg.ExamplesPath,
	}

	writeJSON(w, http.StatusOK, fullResp)
}

func (h *Handler) handleDeprecate(w http.ResponseWriter, r *http.Request, skillID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST allowed")
		return
	}

	var req models.DeprecateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON: "+err.Error())
		return
	}

	if req.Version == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "version is required")
		return
	}

	if err := h.reg.Deprecate(r.Context(), skillID, req.Version); err != nil {
		writeError(w, http.StatusInternalServerError, "deprecate_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"skill_id": skillID,
		"version":  req.Version,
		"status":   "deprecated",
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, models.ApiError{Error: code, Message: message})
}
