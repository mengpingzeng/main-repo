package vault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

const mockKeyVersion = "mock_v1"

// MockEncryptor 使用内存中的 AES-256-GCM 密钥实现 Encryptor 接口。
// 每个 MockEncryptor 实例生成独立的随机密钥（不持久化）。
// 重启后密钥丢失，所有之前加密的数据无法解密——这正是 mock 模式的预期行为。
type MockEncryptor struct {
	key []byte
}

// NewMockEncryptor 创建 Mock 加密器（零配置，自动生成随机密钥）。
func NewMockEncryptor() *MockEncryptor {
	key := make([]byte, 32)
	rand.Read(key)
	return &MockEncryptor{key: key}
}

// NewMockEncryptorWithKey 用指定密钥创建（用于跨测试复现场景）。
func NewMockEncryptorWithKey(key []byte) *MockEncryptor {
	return &MockEncryptor{key: key}
}

func (e *MockEncryptor) Encrypt(ctx context.Context, plaintext []byte) (*EncryptResult, error) {
	block, _ := aes.NewCipher(e.key)
	gcm, _ := cipher.NewGCM(block)

	nonce := make([]byte, gcm.NonceSize())
	io.ReadFull(rand.Reader, nonce)

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return &EncryptResult{
		Ciphertext: ciphertext,
		KeyVersion: mockKeyVersion,
	}, nil
}

func (e *MockEncryptor) Decrypt(ctx context.Context, ciphertext []byte, keyVersion string) ([]byte, error) {
	if keyVersion != "" && keyVersion != mockKeyVersion {
		return nil, fmt.Errorf("unsupported key_version: %s", keyVersion)
	}

	block, _ := aes.NewCipher(e.key)
	gcm, _ := cipher.NewGCM(block)

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, cipherData := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptFailed, err)
	}

	return plaintext, nil
}

func (e *MockEncryptor) Health(ctx context.Context) error {
	return nil
}

func (e *MockEncryptor) Close() error {
	return nil
}

// FailingMockEncryptor 用于测试错误路径的 Encryptor。
// Encrypt 和 Decrypt 都返回预设错误。
type FailingMockEncryptor struct {
	EncryptError error
	DecryptError error
}

func (e *FailingMockEncryptor) Encrypt(ctx context.Context, plaintext []byte) (*EncryptResult, error) {
	if e.EncryptError != nil {
		return nil, e.EncryptError
	}
	return nil, fmt.Errorf("encrypt not implemented in failing mock")
}

func (e *FailingMockEncryptor) Decrypt(ctx context.Context, ciphertext []byte, keyVersion string) ([]byte, error) {
	if e.DecryptError != nil {
		return nil, e.DecryptError
	}
	return nil, fmt.Errorf("decrypt not implemented in failing mock")
}

func (e *FailingMockEncryptor) Health(ctx context.Context) error {
	return nil
}

func (e *FailingMockEncryptor) Close() error {
	return nil
}
