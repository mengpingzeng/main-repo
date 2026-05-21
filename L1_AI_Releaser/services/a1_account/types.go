// Package a1_account 提供 A1 模块的类型定义。
package a1_account

// BindRequest 绑定账号凭证的请求。
type BindRequest struct {
	UID                  string
	Platform             string
	CredentialsPlaintext string
	MaskedDisplay        string
	Caller               string
}

// BindResponse 绑定结果。
type BindResponse struct {
	AccountID     string
	UID           string
	Platform      string
	MaskedDisplay string
	IsNewBinding  bool
	BoundAt       string
}

// GetCredentialsRequest 获取解密凭证的请求。
// 仅 C1 发布模块的服务账号可调用。
// 双重权限校验：Caller 身份 + UID 归属。
type GetCredentialsRequest struct {
	AccountID string
	UID       string
	Caller    string
}

// GetCredentialsResponse 解密后的凭证。
type GetCredentialsResponse struct {
	AccountID       string `json:"account_id"`
	UID             string `json:"uid"`
	Platform        string `json:"platform"`
	Credentials     string `json:"credentials"`
	SecurityWarning string `json:"security_warning"`
	MaskedDisplay   string `json:"masked_display"`
}

// GetCredentialsBatchRequest 批量获取解密凭证的请求。
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
	MaskedDisplay   string `json:"masked_display"`
	Error           string `json:"error"`
}
