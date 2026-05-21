package vault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

const keyVersionV1 = "v1"

// AESEncryptor 使用 AES-256-GCM 进行本地加解密。
// 适用于 mock 模式和开发联调阶段。
type AESEncryptor struct {
	key []byte
}

// NewAESEncryptor 创建 AES 加密器。
// base64Key 是 Base64 编码的 32 字节密钥（来自 A1_MOCK_ENCRYPTION_KEY）。
func NewAESEncryptor(base64Key string) (*AESEncryptor, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("AES-256 requires 32-byte key, got %d bytes", len(key))
	}
	return &AESEncryptor{key: key}, nil
}

// Encrypt 使用 AES-256-GCM 加密明文。
func (e *AESEncryptor) Encrypt(ctx context.Context, plaintext []byte) (*EncryptResult, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return &EncryptResult{
		Ciphertext: ciphertext,
		KeyVersion: keyVersionV1,
	}, nil
}

// Decrypt 使用 AES-256-GCM 解密密文。
func (e *AESEncryptor) Decrypt(ctx context.Context, ciphertext []byte, keyVersion string) ([]byte, error) {
	if keyVersion != "" && keyVersion != keyVersionV1 {
		return nil, fmt.Errorf("unsupported key_version: %s (only %s supported)", keyVersion, keyVersionV1)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short: %d bytes", len(ciphertext))
	}

	nonce, cipherData := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

func (e *AESEncryptor) Health(ctx context.Context) error {
	return nil
}

func (e *AESEncryptor) Close() error {
	return nil
}
