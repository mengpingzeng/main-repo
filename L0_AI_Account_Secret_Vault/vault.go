package vault

import "context"

// SecretVault 是模块的唯一对外接口。
// BFF 通过此接口做账号绑定、列表查询和解绑。
// C1 发布模块通过此接口获取解密凭证（权限受控）。
//
// 非功能语义速查：
//
//	Bind:              幂等（覆盖更新），超时 3s，KMS 故障可重试
//	Unbind:            幂等（重复删除无副作用），超时 2s
//	List:              只读幂等，超时 2s
//	GetCredentials:    只读幂等，超时 5s，KMS 故障可重试
//	GetCredentialsBatch: 只读幂等，超时 10s
type SecretVault interface {
	// Bind 绑定（或更新）一个平台的账号凭证。
	// 幂等：每次调用生成新的 account_id（同平台可绑定多个账号）。
	// 如显式传入 account_id 则覆盖更新已有账号，IsNewBinding=false。
	// 通过 IsNewBinding 区分首次绑定和更新凭证。
	Bind(ctx context.Context, req BindRequest) (*BindResponse, error)

	// Unbind 解绑一个平台的账号凭证。
	// 幂等：重复解绑无副作用，软删除保留审计记录。
	Unbind(ctx context.Context, req UnbindRequest) (*UnbindResponse, error)

	// List 列出用户已绑定的账号列表（支持分页）。
	// 不返回任何凭证字段（连密文都不给），仅返回脱敏展示信息。
	List(ctx context.Context, req ListRequest) (*ListResponse, error)

	// GetCredentials 获取解密后的凭证明文。
	// 双重权限校验：Caller 身份 + UID 归属。
	// 返回值含 SecurityWarning 标记，下游可做静态检查/运行时告警。
	GetCredentials(ctx context.Context, req GetCredentialsRequest) (*GetCredentialsResponse, error)

	// GetCredentialsBatch 批量获取解密凭证。
	// 部分成功语义：单个条目失败不影响其他条目，调用方逐条检查 Error 字段。
	GetCredentialsBatch(ctx context.Context, req GetCredentialsBatchRequest) (*GetCredentialsBatchResponse, error)

	// GetCredentialForOwner 用户自取自己账号的解密凭证（用于 Cookie 注入回浏览器）。
	// 仅校验 UID 归属，不需要 c1_publisher caller 权限。
	// 适用场景：前端「打开番茄」功能，将 Vault 中的 Cookie 注入浏览器以恢复登录态。
	GetCredentialForOwner(ctx context.Context, accountID, uid string) (*GetCredentialsResponse, error)

	// CheckCookieHealth 检测账号 Cookie（登录凭证）是否仍在有效期内。
	// 通过解析平台特定的 session cookie 中的过期时间实现，无需外部网络请求。
	// 若平台尚未实现检测逻辑，返回 ErrPlatformNotSupported（HTTP 422）。
	CheckCookieHealth(ctx context.Context, req CheckCookieHealthRequest) (*CheckCookieHealthResponse, error)

	// Health 返回模块健康状态。nil = 健康。
	Health(ctx context.Context) error

	// Close 优雅关闭，释放 KMS 连接等资源。
	Close() error
}
