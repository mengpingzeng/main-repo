package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"

	a1vault "L0_AI_Account_Secret_Vault"
	"clawstudios/l1_ai_releaser/services/c1_publisher"

	"claw_studios/L2_AI_Interval/internal/adapter"
	"claw_studios/L2_AI_Interval/internal/cycle"
)

var db *sql.DB

func main() {
	dsn := os.Getenv("SCHEDULER_DB_DSN")
	if dsn == "" {
		fmt.Println("❌ SCHEDULER_DB_DSN 未设置")
		os.Exit(1)
	}

	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		fatalf("打开 SCHEDULER DB 失败: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		fatalf("SCHEDULER DB Ping 失败: %v", err)
	}
	fmt.Println("✅ SCHEDULER DB 连接成功")

	vCfg, err := a1vault.LoadConfig()
	if err != nil {
		fatalf("加载 A1 Config 失败: %v", err)
	}
	v, err := a1vault.NewRealSecretVault(vCfg)
	if err != nil {
		fatalf("创建 A1 Vault 失败: %v", err)
	}
	defer v.Close()
	if err := v.Health(context.Background()); err != nil {
		fatalf("A1 Vault 健康检查失败: %v", err)
	}
	fmt.Println("✅ A1 Vault 就绪")

	test1_CreateTable()
	test2_CheckPublishRecord(v)
	test3_RunAndVerify(v)
	test4_CheckFieldValues()
	test5_Idempotency(v)

	fmt.Println("\n═══════════════════════════════════")
	fmt.Println("  全部自测完成")
	fmt.Println("═══════════════════════════════════")
}

func test1_CreateTable() {
	fmt.Print("\n[测试1] 建 platform_stats 表（幂等）... ")
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS platform_stats (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		post_id VARCHAR(128) NOT NULL,
		platform VARCHAR(32) NOT NULL,
		views INT DEFAULT 0,
		likes INT DEFAULT 0,
		comments INT DEFAULT 0,
		shares INT DEFAULT 0,
		snapshot_at TIMESTAMP NOT NULL,
		INDEX idx_post_id (post_id),
		INDEX idx_snapshot (snapshot_at)
	)`)
	if err != nil {
		fmt.Printf("❌ 建表失败: %v\n", err)
		return
	}
	fmt.Println("✅ 表已就绪（重复建不报错）")
}

func test2_CheckPublishRecord(v a1vault.SecretVault) {
	fmt.Print("\n[测试2] 确认 publish_record 有可扫描的 post 且可拿到凭证... ")

	var count int
	err := db.QueryRow(`SELECT COUNT(1) FROM publish_record
		WHERE status = 'ok'
		  AND post_id IS NOT NULL AND post_id != ''
		  AND platform = 'fanqie'
		  AND published_at > ?`, time.Now().AddDate(0, 0, -30)).Scan(&count)
	if err != nil {
		fmt.Printf("❌ 查询失败: %v\n", err)
		return
	}
	if count == 0 {
		fmt.Println("❌ 未找到可扫描的 fanqie post")
		return
	}

	var postID, novelName, accountID, uid string
	err = db.QueryRow(`SELECT post_id, novel_name, account_id, uid FROM publish_record
		WHERE status = 'ok' AND platform = 'fanqie'
		  AND post_id IS NOT NULL AND post_id != ''
		  AND published_at > ?
		LIMIT 1`, time.Now().AddDate(0, 0, -30)).Scan(&postID, &novelName, &accountID, &uid)
	if err != nil {
		fmt.Printf("❌ 读取 publish_record 字段失败: %v\n", err)
		return
	}

	credResp, err := v.GetCredentials(context.Background(), a1vault.GetCredentialsRequest{
		AccountID: accountID,
		UID:       uid,
		Caller:    "c1_publisher",
	})
	if err != nil {
		fmt.Printf("❌ A1 GetCredentials 失败 (account=%s): %v\n", accountID, err)
		return
	}

	fmt.Printf("✅ 可扫描 post=%s novel=%s account=%s credential_len=%d\n",
		postID, novelName, accountID, len(credResp.Credentials))
}

func test3_RunAndVerify(v a1vault.SecretVault) {
	fmt.Print("\n[测试3] 注入真实 FanqieStatsAdapter，执行 Runner.Run()，验证写入... ")

	truncatePlatformStats()

	adp, err := buildFanqieAdapter(v)
	if err != nil {
		fmt.Printf("❌ 构建 FanqieStatsAdapter 失败: %v\n", err)
		return
	}

	runner := cycle.NewRunner(db, adp, cycle.Config{
		BatchSize:     1,
		FetchTimeout:  120 * time.Second,
		BatchInterval: 200 * time.Millisecond,
		MaxRetry:      0,
		RetryBackoff:  1 * time.Second,
	})

	err = runner.Run(context.Background())
	if err != nil {
		fmt.Printf("❌ Runner.Run() 失败: %v\n", err)
		return
	}

	var count int
	db.QueryRow(`SELECT COUNT(1) FROM platform_stats`).Scan(&count)
	if count == 0 {
		fmt.Println("❌ platform_stats 无任何记录")
		return
	}
	fmt.Printf("✅ platform_stats 写入了 %d 条记录\n", count)
}

func test4_CheckFieldValues() {
	fmt.Print("\n[测试4] 查 platform_stats，确认 views/likes/comments/shares 有值... ")

	var views, likes, comments, shares int
	err := db.QueryRow(`SELECT views, likes, comments, shares
		FROM platform_stats ORDER BY id DESC LIMIT 1`).Scan(&views, &likes, &comments, &shares)
	if err != nil {
		fmt.Printf("❌ 查询失败: %v\n", err)
		return
	}

	ok := true
	if views == 0 {
		fmt.Print(" views=0")
		ok = false
	}
	if likes == 0 {
		fmt.Print(" likes=0")
		ok = false
	}
	if comments == 0 {
		fmt.Print(" comments=0")
		ok = false
	}
	if shares == 0 {
		fmt.Print(" shares=0")
		ok = false
	}
	if !ok {
		fmt.Println(" ❌ 部分字段为 0")
		return
	}
	fmt.Printf(" ✅ views=%d likes=%d comments=%d shares=%d\n", views, likes, comments, shares)
}

func test5_Idempotency(v a1vault.SecretVault) {
	fmt.Print("\n[测试5] 再次执行 Runner.Run()，验证幂等（同周期不重复写入）... ")

	var before int
	db.QueryRow(`SELECT COUNT(1) FROM platform_stats`).Scan(&before)

	adp, err := buildFanqieAdapter(v)
	if err != nil {
		fmt.Printf("❌ 构建 FanqieStatsAdapter 失败: %v\n", err)
		return
	}

	runner := cycle.NewRunner(db, adp, cycle.Config{
		BatchSize:     1,
		FetchTimeout:  120 * time.Second,
		BatchInterval: 200 * time.Millisecond,
		MaxRetry:      0,
		RetryBackoff:  1 * time.Second,
	})

	err = runner.Run(context.Background())
	if err != nil {
		fmt.Printf("❌ Runner.Run() 失败: %v\n", err)
		return
	}

	var after int
	db.QueryRow(`SELECT COUNT(1) FROM platform_stats`).Scan(&after)

	if after != before {
		fmt.Printf("❌ 记录数从 %d 变为 %d（幂等失效）\n", before, after)
		return
	}
	fmt.Printf("✅ 记录数不变（%d 条），幂等通过\n", after)
}

// fanqieBridge 将 FanqieStatsAdapter (C1 接口) 适配为 adapter.StatsAdapter (L2 接口)。
type fanqieBridge struct {
	real        *c1_publisher.FanqieStatsAdapter
	credentials string
	platform    string
}

func (b *fanqieBridge) Fetch(ctx context.Context, postID string, _ string) (*adapter.PlatformStats, error) {
	stats, err := b.real.FetchStats(ctx, postID, b.credentials)
	if err != nil {
		return nil, err
	}
	return &adapter.PlatformStats{
		PostID:   postID,
		Platform: b.platform,
		Views:    stats.Views,
		Likes:    stats.Likes,
		Comments: stats.Comments,
		Shares:   stats.Shares,
	}, nil
}

func buildFanqieAdapter(v a1vault.SecretVault) (*fanqieBridge, error) {
	var accountID, novelName, uid string
	err := db.QueryRow(`SELECT account_id, novel_name, uid FROM publish_record
		WHERE status = 'ok' AND platform = 'fanqie'
		  AND post_id IS NOT NULL AND post_id != ''
		  AND published_at > ?
		LIMIT 1`, time.Now().AddDate(0, 0, -30)).Scan(&accountID, &novelName, &uid)
	if err != nil {
		return nil, fmt.Errorf("read publish_record: %w", err)
	}

	credResp, err := v.GetCredentials(context.Background(), a1vault.GetCredentialsRequest{
		AccountID: accountID,
		UID:       uid,
		Caller:    "c1_publisher",
	})
	if err != nil {
		return nil, fmt.Errorf("A1 GetCredentials: %w", err)
	}

	scriptPath := "/home/claw_studios/code/L1_AI_Releaser/scripts/fetch_fanqie_stats.js"
	real := c1_publisher.NewFanqieStatsAdapter(novelName, scriptPath)

	return &fanqieBridge{
		real:        real,
		credentials: credResp.Credentials,
		platform:    "fanqie",
	}, nil
}

func truncatePlatformStats() {
	db.Exec(`DELETE FROM platform_stats`)
}

func fatalf(format string, args ...interface{}) {
	fmt.Printf("❌ "+format+"\n", args...)
	os.Exit(1)
}
