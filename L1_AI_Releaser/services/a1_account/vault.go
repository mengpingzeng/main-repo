// Package a1_account 是 A1 账号凭证模块的接口桩。
// 仅定义 C1 发布模块依赖的接口和类型。
// 真实实现见 services/a1_account/，此处是零外部依赖的最小桩。
package a1_account

import "context"

// SecretVault A1 凭证库接口（仅 C1 需要的部分）。
type SecretVault interface {
	Bind(ctx context.Context, req BindRequest) (*BindResponse, error)
	GetCredentials(ctx context.Context, req GetCredentialsRequest) (*GetCredentialsResponse, error)
	GetCredentialsBatch(ctx context.Context, req GetCredentialsBatchRequest) (*GetCredentialsBatchResponse, error)
	Health(ctx context.Context) error
	Close() error
}
