package vault

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config 是 A1 账号凭证加密存储的完整配置。
type Config struct {
	EncryptorMode string

	MockEncryptionKey string

	KMSKeyID     string
	KMSEndpoint  string
	KMSRegion    string

	C1CallerIdentifier string
	CallerAuthMode     string

	JWTSecret string

	DBHost            string
	DBPort            int
	DBUser            string
	DBPassword        string
	DBName            string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration

	AuditEnabled bool

	Mode string
}

// LoadConfig 从环境变量加载配置，缺失字段用默认值填充。
func LoadConfig() (*Config, error) {
	cfg := &Config{
		EncryptorMode:       getEnv("A1_ENCRYPTOR_MODE", "mock"),
		MockEncryptionKey:   os.Getenv("A1_MOCK_ENCRYPTION_KEY"),
		KMSKeyID:            os.Getenv("A1_KMS_KEY_ID"),
		KMSEndpoint:         os.Getenv("A1_KMS_ENDPOINT"),
		KMSRegion:           getEnv("A1_KMS_REGION", "us-east-1"),
		C1CallerIdentifier:  getEnv("A1_C1_CALLER_ID", "c1_publisher"),
		CallerAuthMode:      getEnv("A1_CALLER_AUTH_MODE", "string"),
		JWTSecret:           getEnv("A1_JWT_SECRET", "change-me-in-production"),
		DBHost:              getEnv("A1_DB_HOST", "127.0.0.1"),
		DBPort:              getEnvInt("A1_DB_PORT", 3306),
		DBUser:              getEnv("A1_DB_USER", "xlongxia"),
		DBPassword:          os.Getenv("A1_DB_PASSWORD"),
		DBName:              getEnv("A1_DB_NAME", "xlongxia"),
		DBMaxOpenConns:      getEnvInt("A1_DB_MAX_OPEN_CONNS", 10),
		DBMaxIdleConns:      getEnvInt("A1_DB_MAX_IDLE_CONNS", 5),
		DBConnMaxLifetime:   getEnvDuration("A1_DB_CONN_MAX_LIFETIME", 300*time.Second),
		AuditEnabled:        getEnvBool("A1_AUDIT_ENABLED", true),
		Mode:                getEnv("A1_ADAPTER_MODE", "real"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// Validate 验证配置合法性。
func (c *Config) Validate() error {
	if c.Mode != "mock" && c.Mode != "real" {
		return fmt.Errorf("mode must be 'mock' or 'real', got: %s", c.Mode)
	}

	if c.EncryptorMode != "mock" && c.EncryptorMode != "kms" {
		return fmt.Errorf("encryptor_mode must be 'mock' or 'kms', got: %s", c.EncryptorMode)
	}

	if c.EncryptorMode == "mock" {
		if c.MockEncryptionKey == "" {
			return fmt.Errorf("A1_MOCK_ENCRYPTION_KEY is required when encryptor_mode=mock")
		}
		keyBytes, err := base64.StdEncoding.DecodeString(c.MockEncryptionKey)
		if err != nil || len(keyBytes) != 32 {
			return fmt.Errorf("A1_MOCK_ENCRYPTION_KEY must be a Base64-encoded 32-byte key")
		}
	}

	if c.EncryptorMode == "kms" {
		if c.KMSKeyID == "" {
			return fmt.Errorf("A1_KMS_KEY_ID is required when encryptor_mode=kms — refusing to run without KMS key")
		}
	}

	if c.Mode == "real" && c.DBPassword == "" {
		return fmt.Errorf("A1_DB_PASSWORD is required when mode=real")
	}

	if c.Mode == "real" && (c.JWTSecret == "" || c.JWTSecret == "change-me-in-production") {
		return fmt.Errorf("A1_JWT_SECRET must be set to a non-default value in production")
	}

	return nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}

// GenerateMockEncryptionKey 生成随机 Mock 加密密钥（Base64 编码的 32 字节）。
func GenerateMockEncryptionKey() string {
	key := make([]byte, 32)
	rand.Read(key)
	return base64.StdEncoding.EncodeToString(key)
}
