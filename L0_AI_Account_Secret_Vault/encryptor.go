package vault

import (
	"context"
	"fmt"
)

// Encryptor 是加密/解密策略的抽象。
// 支持三种实现：
//   - AESEncryptor（mock 模式，本地 AES-256-GCM）
//   - KMSEncryptor（生产模式，云 KMS 服务）
//   - EnvelopeEncryptor（未来模式，Envelope Encryption）
type Encryptor interface {
	Encrypt(ctx context.Context, plaintext []byte) (*EncryptResult, error)
	Decrypt(ctx context.Context, ciphertext []byte, keyVersion string) ([]byte, error)
	Health(ctx context.Context) error
	Close() error
}

// EncryptResult 加密结果。
type EncryptResult struct {
	Ciphertext []byte
	KeyVersion string
}

// NewEncryptor 根据配置创建对应的 Encryptor 实现。
func NewEncryptor(cfg *Config) (Encryptor, error) {
	switch cfg.EncryptorMode {
	case "mock":
		return NewAESEncryptor(cfg.MockEncryptionKey)
	case "kms":
		client, err := NewKMSSDKClient(cfg)
		if err != nil {
			return nil, fmt.Errorf("init KMS client: %w", err)
		}
		return NewKMSEncryptor(cfg.KMSKeyID, cfg.KMSEndpoint, cfg.KMSRegion, client), nil
	default:
		return nil, fmt.Errorf("unsupported encryptor mode: %s", cfg.EncryptorMode)
	}
}
