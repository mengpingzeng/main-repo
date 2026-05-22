package vault

// RegisterRequest 用户注册请求。
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// LoginRequest 用户登录请求。
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse 认证响应。
type AuthResponse struct {
	UID      string `json:"uid"`
	Username string `json:"username"`
	Token    string `json:"token"`
	Role     string `json:"role"`
}

// AdminUserInfo 管理员查看的用户信息（含统计数据）。
type AdminUserInfo struct {
	UID          string `json:"uid"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	AccountCount int    `json:"accountCount"`
	TaskCount    int    `json:"taskCount"`
	CreatedAt    string `json:"createdAt"`
	LastLoginAt  string `json:"lastLoginAt,omitempty"`
}

// AdminUserListResponse 用户列表响应。
type AdminUserListResponse struct {
	Users []AdminUserInfo `json:"users"`
}

// CreateUserRequest 管理员创建用户请求。
type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// CreateUserResponse 创建用户响应。
type CreateUserResponse struct {
	UID       string `json:"uid"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt"`
}

// UpdateUserRequest 管理员修改用户请求。
type UpdateUserRequest struct {
	Password string `json:"password"`
	Role     string `json:"role"`
}

// UpdateUserResponse 修改用户响应。
type UpdateUserResponse struct {
	UID       string `json:"uid"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	UpdatedAt string `json:"updatedAt"`
}

// DeleteUserResponse 删除用户响应。
type DeleteUserResponse struct {
	Deleted bool `json:"deleted"`
}

// BindRequest 绑定账号凭证的请求。
type BindRequest struct {
	UID string `json:"uid"`

	Platform string `json:"platform"`

	CredentialsPlaintext string `json:"credentials_plaintext"`

	// MaskedDisplay 前端展示用的脱敏标识。
	// 如为空，模块自动从 credentials 中按平台契约提取脱敏字段生成（见 ADR-009）。
	MaskedDisplay string `json:"masked_display"`

	// AccountID 调用方指定账号 ID（可选）。
	// 如非空则使用调用方传入的值，否则自动生成。
	AccountID string `json:"account_id"`

	// Caller 调用方标识，用于审计日志。
	// 注意：P0 阶段为裸字符串（存在伪造风险），P2 加固为 mTLS。
	Caller string `json:"caller"`
}

// BindResponse 绑定结果。
type BindResponse struct {
	AccountID string `json:"account_id"`

	UID string `json:"uid"`

	Platform string `json:"platform"`

	MaskedDisplay string `json:"masked_display"`

	// IsNewBinding 是否为新绑定。
	//   true: 首次绑定该平台（前端可提示"绑定成功"）
	//   false: 覆盖更新已有凭证（前端可提示"凭证已更新"）
	IsNewBinding bool `json:"is_new_binding"`

	// BoundAt 绑定/更新时间，ISO8601 UTC。
	BoundAt string `json:"bound_at"`
}

// UnbindRequest 解绑账号凭证的请求。
type UnbindRequest struct {
	AccountID string `json:"account_id"`

	UID string `json:"uid"`

	Caller string `json:"caller"`
}

// UnbindResponse 解绑结果。
type UnbindResponse struct {
	AccountID string `json:"account_id"`

	// UnboundAt 解绑时间，ISO8601 UTC。
	UnboundAt string `json:"unbound_at"`
}

// ListRequest 查询已绑账号列表的请求（支持分页）。
type ListRequest struct {
	UID string `json:"uid"`

	// Platform 平台过滤（可选），为空则返回所有平台。
	Platform string `json:"platform"`

	// Offset 分页偏移量（从 0 开始）。
	Offset int `json:"offset"`

	// Limit 每页数量（默认 20，最大 100）。
	Limit int `json:"limit"`
}

// ListResponse 账号列表响应。
type ListResponse struct {
	Accounts []AccountSummary `json:"accounts"`
	Total    int              `json:"total"`
}

// AccountSummary 账号摘要（仅脱敏信息，不含凭证）。
type AccountSummary struct {
	AccountID     string `json:"account_id"`
	UID           string `json:"uid"`
	Platform      string `json:"platform"`
	MaskedDisplay string `json:"masked_display"`
	BoundAt       string `json:"bound_at"`
	UpdatedAt     string `json:"updated_at"`
}

// GetCredentialsRequest 获取解密凭证的请求。
// 仅 C1 发布模块的服务账号可调用。
// 双重权限校验：Caller 身份 + UID 归属。
type GetCredentialsRequest struct {
	AccountID string `json:"account_id"`

	// UID 用户唯一标识，用于归属校验。
	UID string `json:"uid"`

	// Caller 调用方标识，必须为 "c1_publisher" 否则拒绝。
	Caller string `json:"caller"`
}

// GetCredentialsResponse 解密后的凭证。
// SecurityWarning 字段供下游系统做可编程安全标记（见 ADR-007）。
type GetCredentialsResponse struct {
	AccountID       string `json:"account_id"`
	UID             string `json:"uid"`
	Platform        string `json:"platform"`
	Credentials     string `json:"credentials"`
	MaskedDisplay   string `json:"masked_display"`
	// SecurityWarning 安全警告标记，固定文本 "SENSITIVE: DO NOT LOG"。
	SecurityWarning string `json:"security_warning"`
}

// GetCredentialsBatchRequest 批量获取解密凭证的请求。
// 部分成功语义：单个条目失败不影响其他条目。
type GetCredentialsBatchRequest struct {
	AccountIDs []string `json:"account_ids"`
	UID        string   `json:"uid"`
	Caller     string   `json:"caller"`
}

// GetCredentialsBatchResponse 批量解密结果。
type GetCredentialsBatchResponse struct {
	Results []CredentialsResult `json:"results"`
}

// CredentialsResult 单个账号的凭证结果。
type CredentialsResult struct {
	AccountID       string `json:"account_id"`
	UID             string `json:"uid"`
	Platform        string `json:"platform"`
	Credentials     string `json:"credentials"`
	SecurityWarning string `json:"security_warning"`
	Error           string `json:"error"`
}

// CheckCookieHealthRequest 检测账号 Cookie 有效性的请求（内部）。
type CheckCookieHealthRequest struct {
	AccountID string
	// UID 用于归属校验，确保只能检测自己的账号。
	UID string
}

// CheckCookieHealthResponse 登录状态检测结果。
type CheckCookieHealthResponse struct {
	AccountID string `json:"account_id"`
	// Valid true = Cookie 在有效期内；false = 已过期或解析失败。
	Valid     bool   `json:"valid"`
	CheckedAt string `json:"checked_at"`
}
