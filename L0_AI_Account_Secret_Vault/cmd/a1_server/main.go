package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	vault "L0_AI_Account_Secret_Vault"
	"clawstudios/pkg/logging"
)

type server struct {
	vault     vault.SecretVault
	jwtSecret string
	rv        *vault.RealSecretVault
}

func main() {
	cfg, err := vault.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	srv := &server{
		jwtSecret: cfg.JWTSecret,
	}

	if cfg.Mode == "real" {
		rv, err := vault.NewRealSecretVault(cfg)
		if err != nil {
			log.Fatalf("failed to create real vault: %v", err)
		}
		defer rv.Close()
		srv.vault = rv
		srv.rv = rv
		if err := rv.BackfillCredentialFingerprints(context.Background()); err != nil {
			log.Printf("warning: credential fingerprint backfill failed: %v", err)
		}
		if err := rv.BackfillPlatformAuthorIDs(context.Background()); err != nil {
			log.Printf("warning: platform author id backfill failed: %v", err)
		}
	} else {
		srv.vault = vault.NewMockSecretVault()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/register", srv.handleRegister)
	mux.HandleFunc("POST /api/auth/login", srv.handleLogin)
	mux.HandleFunc("POST /api/account/bind", srv.requireAuth(srv.handleBind))
	mux.HandleFunc("POST /api/account/unbind", srv.requireAuth(srv.handleUnbind))
	mux.HandleFunc("GET /api/account/list", srv.requireAuth(srv.handleList))
	mux.HandleFunc("GET /api/account/health/{account_id}", srv.requireAuth(srv.handleCookieHealth))
	mux.HandleFunc("GET /api/account/credential/{account_id}", srv.requireAuth(srv.handleGetCredentialForOwner))
	mux.HandleFunc("POST /api/account/credentials", srv.handleGetCredentials)
	mux.HandleFunc("GET /healthz", srv.handleHealth)

	mux.HandleFunc("GET /api/admin/users", srv.requireAuth(srv.requireAdmin(srv.handleListUsers)))
	mux.HandleFunc("POST /api/admin/users", srv.requireAuth(srv.requireAdmin(srv.handleCreateUser)))
	mux.HandleFunc("PUT /api/admin/users/{uid}", srv.requireAuth(srv.requireAdmin(srv.handleUpdateUser)))
	mux.HandleFunc("DELETE /api/admin/users/{uid}", srv.requireAuth(srv.requireAdmin(srv.handleDeleteUser)))

	port := os.Getenv("A1_LISTEN_PORT")
	if port == "" {
		port = "8084"
	}

	addr := ":" + port
	log.Printf("a1_server listening on %s (mode=%s)", addr, cfg.Mode)
	if err := http.ListenAndServe(addr, logging.HTTPMiddleware("A1AccountVault")(mux)); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func (s *server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := r.Header.Get("X-User-ID")
		role := r.Header.Get("X-User-Role")
		if uid != "" {
			ctx := vault.SetAuthContext(r.Context(), &vault.CustomClaims{UID: uid, Role: role})
			next(w, r.WithContext(ctx))
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "MISSING_AUTH", "authorization header or X-User-ID is required")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", "authorization header must be Bearer <token>")
			return
		}

		c, err := vault.VerifyToken(parts[1], s.jwtSecret)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid or expired token")
			return
		}

		ctx := vault.SetAuthContext(r.Context(), c)
		next(w, r.WithContext(ctx))
	}
}

func (s *server) handleRegister(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context())
	if s.rv == nil {
		writeError(w, http.StatusNotImplemented, "NOT_AVAILABLE", "user auth requires real mode")
		return
	}

	var req vault.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "username and password are required")
		return
	}

	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "INVALID_PASSWORD", "密码至少需要 8 位")
		return
	}

	role := req.Role
	if role == "" {
		role = "user"
	}

	user, err := s.rv.Register(r.Context(), req.Username, req.Password, role)
	if err != nil {
		if logger != nil {
			logger.Error(logging.ErrDatabaseError, "Register user %s failed: %v", req.Username, err)
		}
		writeVaultError(w, err)
		return
	}

	token, err := vault.GenerateToken(user.UID, user.Username, user.Role, s.jwtSecret)
	if err != nil {
		if logger != nil {
			logger.Error(logging.ErrInternal, "GenerateToken for user %s failed: %v", user.UID, err)
		}
		writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "failed to generate token")
		return
	}

	writeJSON(w, http.StatusCreated, vault.AuthResponse{
		UID:      user.UID,
		Username: user.Username,
		Token:    token,
		Role:     user.Role,
	})
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context())
	if s.rv == nil {
		writeError(w, http.StatusNotImplemented, "NOT_AVAILABLE", "user auth requires real mode")
		return
	}

	var req vault.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "username and password are required")
		return
	}

	user, err := s.rv.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		if logger != nil {
			logger.Error(logging.ErrUnauthorized, "Login for user %s failed: %v", req.Username, err)
		}
		writeVaultError(w, err)
		return
	}

	token, err := vault.GenerateToken(user.UID, user.Username, user.Role, s.jwtSecret)
	if err != nil {
		if logger != nil {
			logger.Error(logging.ErrInternal, "GenerateToken for user %s failed: %v", user.UID, err)
		}
		writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, vault.AuthResponse{
		UID:      user.UID,
		Username: user.Username,
		Token:    token,
		Role:     user.Role,
	})
}

func (s *server) handleBind(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context())
	var req vault.BindRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if logger != nil {
			logger.Error(logging.ErrInvalidParam, "decode bind request failed: %v", err)
		}
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	claims := vault.GetAuthClaims(r.Context())
	uid := ""
	role := ""
	if claims != nil {
		uid = claims.UID
		role = claims.Role
	}

	if role == "admin" && req.UID != "" {
		uid = req.UID
	}

	if req.UID != "" && req.UID != uid {
		writeError(w, http.StatusForbidden, "UID_MISMATCH", "request uid does not match authenticated user")
		return
	}
	req.UID = uid

	if req.Caller == "" {
		req.Caller = "bff"
	}

	resp, err := s.vault.Bind(r.Context(), req)
	if err != nil {
		if logger != nil {
			logger.Error(logging.ErrDatabaseError, "Bind account for uid %s failed: %v", uid, err)
		}
		writeVaultError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) handleUnbind(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context())
	var req vault.UnbindRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if logger != nil {
			logger.Error(logging.ErrInvalidParam, "decode unbind request failed: %v", err)
		}
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	claims := vault.GetAuthClaims(r.Context())
	uid := ""
	role := ""
	if claims != nil {
		uid = claims.UID
		role = claims.Role
	}

	if role == "admin" {
		// Admin can unbind any account: bypass UID ownership check
		req.UID = ""
	} else {
		if req.UID != "" && req.UID != uid {
			writeError(w, http.StatusForbidden, "UID_MISMATCH", "request uid does not match authenticated user")
			return
		}
		req.UID = uid
	}

	if req.Caller == "" {
		req.Caller = "bff"
	}

	resp, err := s.vault.Unbind(r.Context(), req)
	if err != nil {
		if logger != nil {
			logger.Error(logging.ErrDatabaseError, "Unbind account for uid %s failed: %v", uid, err)
		}
		writeVaultError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) handleList(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context())
	claims := vault.GetAuthClaims(r.Context())
	uid := ""
	role := ""
	if claims != nil {
		uid = claims.UID
		role = claims.Role
	}

	platform := r.URL.Query().Get("platform")

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		offset, _ = strconv.Atoi(v)
	}

	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	if role == "admin" && r.URL.Query().Get("uid") == "" {
		uid = ""
	}

	req := vault.ListRequest{
		UID:      uid,
		Platform: platform,
		Offset:   offset,
		Limit:    limit,
	}

	resp, err := s.vault.List(r.Context(), req)
	if err != nil {
		if logger != nil {
			logger.Error(logging.ErrDatabaseError, "List accounts for uid %s failed: %v", uid, err)
		}
		writeVaultError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) handleGetCredentials(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context())
	var req vault.GetCredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if logger != nil {
			logger.Error(logging.ErrInvalidParam, "decode credentials request failed: %v", err)
		}
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	resp, err := s.vault.GetCredentials(r.Context(), req)
	if err != nil {
		if logger != nil {
			logger.Error(logging.ErrDatabaseError, "GetCredentials failed: %v", err)
		}
		writeVaultError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context())
	if err := s.vault.Health(r.Context()); err != nil {
		if logger != nil {
			logger.Error(logging.ErrDatabaseError, "Health check failed: %v", err)
		}
		writeError(w, http.StatusServiceUnavailable, "UNHEALTHY", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := vault.GetAuthClaims(r.Context())
		if claims == nil || claims.Role != "admin" {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "admin role required")
			return
		}
		next(w, r)
	}
}

func (s *server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context())
	if s.rv == nil {
		writeError(w, http.StatusNotImplemented, "NOT_AVAILABLE", "user management requires real mode")
		return
	}

	page := 1
	size := 5
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if sz := r.URL.Query().Get("size"); sz != "" {
		if v, err := strconv.Atoi(sz); err == nil && v > 0 {
			size = v
		}
	}

	priorityUID := ""
	if claims := vault.GetAuthClaims(r.Context()); claims != nil {
		priorityUID = claims.UID
	}

	users, total, err := s.rv.ListUsers(r.Context(), page, size, priorityUID)
	if err != nil {
		if logger != nil {
			logger.Error(logging.ErrDatabaseError, "ListUsers failed: %v", err)
		}
		writeVaultError(w, err)
		return
	}

	if users == nil {
		users = []vault.AdminUserInfo{}
	}
	writeJSON(w, http.StatusOK, vault.AdminUserListResponse{Users: users, Total: total})
}

func (s *server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context())
	if s.rv == nil {
		writeError(w, http.StatusNotImplemented, "NOT_AVAILABLE", "user management requires real mode")
		return
	}

	var req vault.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if logger != nil {
			logger.Error(logging.ErrInvalidParam, "decode create user request failed: %v", err)
		}
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "username and password are required")
		return
	}

	if req.Role != "admin" && req.Role != "user" && req.Role != "" {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "role must be 'admin' or 'user'")
		return
	}

	role := req.Role
	if role == "" {
		role = "user"
	}

	user, err := s.rv.Register(r.Context(), req.Username, req.Password, role)
	if err != nil {
		if logger != nil {
			logger.Error(logging.ErrDatabaseError, "Register user %s failed: %v", req.Username, err)
		}
		writeVaultError(w, err)
		return
	}

	operatorUID := ""
	if claims := vault.GetAuthClaims(r.Context()); claims != nil {
		operatorUID = claims.UID
	}
	s.rv.RecordAdminAudit(r.Context(), operatorUID, "create_user", user.UID, "role: "+role)

	writeJSON(w, http.StatusCreated, vault.CreateUserResponse{
		UID:       user.UID,
		Username:  user.Username,
		Role:      user.Role,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
	})
}

func (s *server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context())
	if s.rv == nil {
		writeError(w, http.StatusNotImplemented, "NOT_AVAILABLE", "user management requires real mode")
		return
	}

	uid := r.PathValue("uid")
	if uid == "" {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "user uid is required")
		return
	}

	var req vault.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if logger != nil {
			logger.Error(logging.ErrInvalidParam, "decode update user request failed: %v", err)
		}
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	operatorUID := ""
	claims := vault.GetAuthClaims(r.Context())
	if claims != nil {
		operatorUID = claims.UID
	}

	if err := s.rv.UpdateUser(r.Context(), uid, req.Password, req.Role, operatorUID); err != nil {
		if logger != nil {
			logger.Error(logging.ErrDatabaseError, "UpdateUser(%s) failed: %v", uid, err)
		}
		writeVaultError(w, err)
		return
	}

	detail := ""
	if req.Password != "" {
		detail = "password_reset"
	}
	if req.Role != "" {
		if detail != "" {
			detail += ", "
		}
		detail += "role: " + req.Role
	}
	s.rv.RecordAdminAudit(r.Context(), operatorUID, "update_user", uid, detail)

	writeJSON(w, http.StatusOK, vault.UpdateUserResponse{
		UID:       uid,
		Username:  "",
		Role:      req.Role,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context())
	if s.rv == nil {
		writeError(w, http.StatusNotImplemented, "NOT_AVAILABLE", "user management requires real mode")
		return
	}

	uid := r.PathValue("uid")
	if uid == "" {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "user uid is required")
		return
	}

	operatorUID := ""
	if claims := vault.GetAuthClaims(r.Context()); claims != nil {
		operatorUID = claims.UID
	}

	if err := s.rv.DeleteUser(r.Context(), uid, operatorUID); err != nil {
		if logger != nil {
			logger.Error(logging.ErrDatabaseError, "DeleteUser(%s) failed: %v", uid, err)
		}
		writeVaultError(w, err)
		return
	}

	s.rv.RecordAdminAudit(r.Context(), operatorUID, "delete_user", uid, "")
	writeJSON(w, http.StatusOK, vault.DeleteUserResponse{Deleted: true})
}

func (s *server) handleCookieHealth(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context())
	accountID := r.PathValue("account_id")
	if accountID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "account_id is required")
		return
	}

	uid := ""
	if claims := vault.GetAuthClaims(r.Context()); claims != nil {
		// admin 角色可检测任意账号，传空 UID 跳过归属校验
		if claims.Role != "admin" {
			uid = claims.UID
		}
	}

	resp, err := s.vault.CheckCookieHealth(r.Context(), vault.CheckCookieHealthRequest{
		AccountID: accountID,
		UID:       uid,
	})
	if err != nil {
		if logger != nil {
			logger.Error(logging.ErrExternalService, "CheckCookieHealth(%s) failed: %v", accountID, err)
		}
		writeVaultError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) handleGetCredentialForOwner(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context())
	accountID := r.PathValue("account_id")
	if accountID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "account_id is required")
		return
	}

	uid := ""
	if claims := vault.GetAuthClaims(r.Context()); claims != nil {
		// admin 角色可操作任意账号，传空 UID 跳过归属校验
		if claims.Role != "admin" {
			uid = claims.UID
		}
	}

	resp, err := s.vault.GetCredentialForOwner(r.Context(), accountID, uid)
	if err != nil {
		if logger != nil {
			logger.Error(logging.ErrDatabaseError, "GetCredentialForOwner(%s) failed: %v", accountID, err)
		}
		writeVaultError(w, err)
		return
	}
	// 明文凭证不写入任何日志，直接返回给调用方
	writeJSON(w, http.StatusOK, resp)
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Code: code, Message: message})
}

func writeVaultError(w http.ResponseWriter, err error) {
	status := vault.HTTPStatusCode(err)
	code := vault.ErrorCode(err)
	msg := err.Error()
	var se *vault.SecretError
	if errors.As(err, &se) {
		msg = se.Message
	}
	writeError(w, status, code, msg)
}
