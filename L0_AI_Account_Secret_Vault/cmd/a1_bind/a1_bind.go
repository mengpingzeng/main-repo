package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"

	vault "L0_AI_Account_Secret_Vault"
)

var (
	platform string
	cookie   string
)

func init() {
	platform = getEnvOrDefault("A1_PLATFORM", "zhulang")
	if cookie = os.Getenv("A1_COOKIE"); cookie != "" {
	} else if data, err := os.ReadFile("/tmp/zhulang_cookie.txt"); err == nil {
		cookie = string(data)
	} else if data, err := os.ReadFile("/tmp/fanqie_cookie.txt"); err == nil {
		cookie = string(data)
	}
	if cookie == "" {
		fmt.Println("no cookie found (A1_COOKIE or /tmp/zhulang_cookie.txt)")
		os.Exit(1)
	}
}

func parseDSN(dsn string) (user, password, host string, port int, dbname string, err error) {
	re := regexp.MustCompile(`^([^:]+):([^@]+)@tcp\(([^:]+):(\d+)\)/([^?]+)`)
	matches := re.FindStringSubmatch(dsn)
	if len(matches) != 6 {
		return "", "", "", 0, "", fmt.Errorf("invalid DSN: %s", dsn)
	}
	user = matches[1]
	password = matches[2]
	host = matches[3]
	port, _ = strconv.Atoi(matches[4])
	dbname = matches[5]
	return
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func main() {
	dsn := os.Getenv("A1_DB_DSN")
	if dsn == "" {
		fmt.Println("A1_DB_DSN not set")
		os.Exit(1)
	}

	encKey := os.Getenv("A1_ENCRYPTION_KEY")
	if encKey == "" {
		fmt.Println("A1_ENCRYPTION_KEY not set")
		os.Exit(1)
	}

	uid := getEnvOrDefault("A1_UID", "user_001")
	jwtSecret := getEnvOrDefault("A1_JWT_SECRET", "change-me-in-production")

	user, password, host, port, dbname, err := parseDSN(dsn)
	if err != nil {
		fmt.Printf("parse DSN failed: %v\n", err)
		os.Exit(1)
	}

	cfg := &vault.Config{
		EncryptorMode:      "mock",
		MockEncryptionKey:  encKey,
		C1CallerIdentifier: "c1_publisher",
		JWTSecret:          jwtSecret,
		DBHost:             host,
		DBPort:             port,
		DBUser:             user,
		DBPassword:         password,
		DBName:             dbname,
		DBMaxOpenConns:     5,
		DBMaxIdleConns:     2,
		DBConnMaxLifetime:  300 * time.Second,
		AuditEnabled:       true,
		Mode:               "real",
	}

	v, err := vault.NewRealSecretVault(cfg)
	if err != nil {
		fmt.Printf("init vault failed: %v\n", err)
		os.Exit(1)
	}
	defer v.Close()

	resp, err := v.Bind(context.Background(), vault.BindRequest{
		UID:                  uid,
		Platform:             platform,
		CredentialsPlaintext: cookie,
		Caller:               "bff",
	})
	if err != nil {
		fmt.Printf("绑定失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("绑定成功 account_id=%s uid=%s platform=%s\n", resp.AccountID, uid, platform)
}
