// cmd/real_demo 演示 C1 发布模块连接真实平台 API。
//
// 通过环境变量配置各平台 API 地址和凭证。
// 默认使用 Mock 模式（不连真实 API），设置环境变量后切换为真实模式。
//
// 运行方式：
//
//	# Mock 模式（默认）
//	go run ./cmd/real_demo/
//
//	# 真实模式（须先配置环境变量）
//	XHS_BASE_URL=https://edith.xiaohongshu.com/api \
//	WECHAT_BASE_URL=https://api.weixin.qq.com \
//	FANQIE_BASE_URL=https://api.fanqienovel.com \
//	A1_MODE=real \
//	go run ./cmd/real_demo/
//
// 环境变量说明：
//
//	XHS_BASE_URL      小红书 API 地址（默认 http://localhost:9100）
//	WECHAT_BASE_URL   公众号 API 地址（默认 http://localhost:9101）
//	FANQIE_BASE_URL   番茄小说 API 地址（默认 http://localhost:9102）
//	ADAPTER_MODE      适配器模式: mock | real（默认 mock）
//	CONCURRENCY_LIMIT 最大并发数（默认 0=不限制）
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"clawstudios/l1_ai_releaser/services/a1_account"
	"clawstudios/l1_ai_releaser/services/c1_publisher"
)

// demoChapterContent 番茄小说测试用章节正文（≥1000 字以通过平台校验）。
const demoChapterContent = `第一章 初入江湖

王小龙睁开眼，发现自己穿越到了一个完全陌生的世界。周围是一片广袤的竹林，竹叶在风中沙沙作响。他的手中握着一本破旧的古籍，封面上写着"小龙虾烹饪秘籍"几个大字。这与他在地球上的职业——小龙虾餐厅大厨——形成了奇妙的呼应。

正当他困惑之际，一道系统提示音在脑海中响起："叮！恭喜宿主激活美食江湖系统！检测到宿主具备精湛的厨艺技能，现发布新手任务：在异世界开一家小龙虾餐厅。"

王小龙愣了一下，随即露出自信的笑容。"做菜？这可是我的拿手好戏！"他立刻起身，开始探索这个名为"美食大陆"的异世界。这里的人以美食为修行之道，不同的食材和烹饪方式可以产生不同的灵力增益。而小龙虾，正是这个世界最珍贵的食材之一，被称为"江湖至味"。

他顺着竹林的羊肠小道前行，很快就看到了一个热闹的小镇。镇上到处都是美食摊位，空气中弥漫着各种奇异的香气。王小龙深吸一口气，"好，就在这里开始我的异世界小龙虾帝国吧！"

接下来的日子里，他利用系统奖励的食材和烹饪工具，在小镇上开了一家名为"暴龙小龙虾"的餐厅。开业第一天，香气四溢的小龙虾就吸引了众多食客。每个人都对这道新奇的菜品赞不绝口，许多武者甚至在食用后感受到明显的灵力提升。

消息很快传开了。"暴龙小龙虾"成了镇上的热门话题。而王小龙也通过系统获得了新的厨艺技能和修炼功法。他知道，这只是他在这片美食大陆上的第一步。前方，还有更多挑战和机遇在等着他。`

func main() {
	ctx := context.Background()
	adapterMode := envOr("ADAPTER_MODE", "mock")

	fmt.Println("========================================")
	fmt.Println("  C1 发布模块 — 平台连接测试")
	fmt.Printf("  适配器模式: %s\n", adapterMode)
	fmt.Println("========================================")
	fmt.Println()

	vault := a1_account.NewMockSecretVault()
	bindAccounts(ctx, vault)
	fmt.Println("✅ 账号绑定完成")

	a1Server := newMockA1Server(vault)
	defer a1Server.Close()

	adapters := createAdapters(adapterMode)
	fmt.Printf("✅ 适配器创建完成 (%s 模式)\n", adapterMode)

	publisher := c1_publisher.NewRealPublisher(c1_publisher.Config{
		A1BaseURL: a1Server.URL,
		Adapters:  adapters,
	})
	fmt.Println("✅ 发布器初始化完成")
	fmt.Println()

	// 4. 发布测试内容
	req := buildDemoRequest()
	fmt.Println("📤 开始发布...")
	startTime := time.Now()

	resp, err := publisher.Publish(ctx, req)

	elapsed := time.Since(startTime)

	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 发布失败: %v\n", err)
		os.Exit(1)
	}

	// 5. 展示结果
	printResults(resp, elapsed)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func bindAccounts(ctx context.Context, vault a1_account.SecretVault) {
	accounts := []struct {
		uid, platform, credentials string
	}{
		{
			uid:    "user_1",
			platform: "xhs",
			credentials: envOr("XHS_CREDENTIALS",
				"a1=xxx; webId=yyy; web_session=zzz"),
		},
		{
			uid:    "user_1",
			platform: "wechat",
			credentials: envOr("WECHAT_CREDENTIALS",
				`{"app_id":"wx_test","app_secret":"test_secret","access_token":"test_token"}`),
		},
		{
			uid:    "user_2",
			platform: "fanqie",
			// 番茄小说使用 cookie（Puppeteer 模式），不是 JSON token
			credentials: envOr("FANQIE_CREDENTIALS",
				"sessionid=test_session_xxx; novel_web_token=yyy"),
		},
		{
			uid:    "user_3",
			platform: "zhulang",
			// 逐浪网使用 cookie（Puppeteer 模式）
			credentials: envOr("ZHULANG_CREDENTIALS",
				"PHPSESSID=test_session_xxx; user_token=yyy"),
		},
	}

	for _, a := range accounts {
		resp, err := vault.Bind(ctx, a1_account.BindRequest{
			UID: a.uid, Platform: a.platform,
			CredentialsPlaintext: a.credentials,
			Caller: "bff",
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "绑定失败 %s/%s: %v\n", a.uid, a.platform, err)
			continue
		}
		fmt.Printf("  📎 %s → %s\n", resp.AccountID, a.platform)
	}
}

func createAdapters(mode string) []c1_publisher.PublishAdapter {
	var adapters []c1_publisher.PublishAdapter

	if mode == "real" {
		xhsCfg := c1_publisher.AdapterConfig{
			BaseURL:        envOr("XHS_BASE_URL", "http://localhost:9100"),
			RequestTimeout: 30 * time.Second,
		}
		wechatCfg := c1_publisher.AdapterConfig{
			BaseURL:        envOr("WECHAT_BASE_URL", "http://localhost:9101"),
			RequestTimeout: 30 * time.Second,
		}
		fanqieCfg := c1_publisher.AdapterConfig{
			ScriptPath: envOr("FANQIE_SCRIPT_PATH", "./scripts/publish_fanqie.js"),
			NodeBin:    envOr("NODE_BIN", "node"),
			Timeout:    90 * time.Second,
		}
		zhulangCfg := c1_publisher.AdapterConfig{
			ScriptPath: envOr("ZHULANG_SCRIPT_PATH", "./scripts/publish_zhulang.js"),
			NodeBin:    envOr("NODE_BIN", "node"),
			Timeout:    300 * time.Second,
		}

		xhs := c1_publisher.NewXhsPublishAdapter(xhsCfg)
		wechat := c1_publisher.NewWechatPublishAdapter(wechatCfg)
		fanqie := c1_publisher.NewFanqiePublishAdapter(fanqieCfg)
		zhulang := c1_publisher.NewZhulangPublishAdapter(zhulangCfg)

		fmt.Printf("  🔗 小红书 → %s\n", xhsCfg.BaseURL)
		fmt.Printf("  🔗 公众号 → %s\n", wechatCfg.BaseURL)
		fmt.Printf("  🔗 番茄小说 → Puppeteer 模式 (脚本: %s)\n", fanqieCfg.ScriptPath)
		fmt.Printf("  🔗 逐浪网 → Puppeteer 模式 (脚本: %s)\n", zhulangCfg.ScriptPath)

		adapters = []c1_publisher.PublishAdapter{xhs, wechat, fanqie, zhulang}
	} else {
		adapters = []c1_publisher.PublishAdapter{
			c1_publisher.NewMockPublishAdapter("xhs"),
			c1_publisher.NewMockPublishAdapter("wechat"),
			c1_publisher.NewMockPublishAdapter("fanqie"),
			c1_publisher.NewMockPublishAdapter("zhulang"),
		}
	}

	return adapters
}

func buildDemoRequest() c1_publisher.PublishRequest {
	return c1_publisher.PublishRequest{
		TaskID: "task_conn_test_001",
		Products: map[string]c1_publisher.ProductContent{
			"xhs": {
				Text: "【连接测试】这是一条自动生成的测试文案。如果你看到这条内容，说明 API 连通正常。",
				Tags: []string{"测试", "连通性"},
			},
			"wechat": {
				HTML: "<h2>连接测试</h2><p>这是一条由 C1 发布模块自动生成的测试文章。</p>",
			},
			"fanqie": {
				Text:          demoChapterContent,
				NovelName:     "穿越之小龙虾传奇",
				VolumeName:    "第一卷",
				ChapterNumber: 1,
				Title:         "第一章 初入江湖",
			},
			"zhulang": {
				Text:          demoChapterContent,
				NovelName:     "穿越之小龙虾传奇",
				VolumeName:    "第一卷",
				ChapterNumber: 1,
				Title:         "第一章 初入江湖",
			},
		},
		Accounts: []c1_publisher.AccountRef{
			{AccountID: "acc_user_1_xhs", UID: "user_1", Platform: "xhs"},
			{AccountID: "acc_user_1_wechat", UID: "user_1", Platform: "wechat"},
			{AccountID: "acc_user_2_fanqie", UID: "user_2", Platform: "fanqie"},
			{AccountID: "acc_user_3_zhulang", UID: "user_3", Platform: "zhulang"},
		},
		TraceID: "trace_conn_test_001",
	}
}

func printResults(resp *c1_publisher.PublishResponse, elapsed time.Duration) {
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  发布结果")
	fmt.Println("========================================")
	fmt.Printf("  总耗时:   %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  总计:     %d 条\n", resp.Summary.Total)
	fmt.Printf("  成功:     %d 条\n", resp.Summary.Succeeded)
	fmt.Printf("  失败:     %d 条\n", resp.Summary.Failed)
	fmt.Println("----------------------------------------")

	for _, r := range resp.Results {
		icon := "✅"
		if r.Status != "ok" {
			icon = "❌"
		}
		fmt.Printf("  %s %-8s │ %-20s │ %s\n",
			icon, r.Platform, r.AccountID, r.Status)
		fmt.Printf("           post_id: %s\n", orNA(r.PostID))
		if r.ErrorCode != "" {
			fmt.Printf("           error:   %s — %s\n", r.ErrorCode, r.ErrorMessage)
		}
	}
	fmt.Println("========================================")

	// 诊断建议
	if resp.Summary.Failed > 0 {
		fmt.Println()
		fmt.Println("🔍 故障诊断：")
		fmt.Println()
		for _, r := range resp.Results {
			switch {
			case r.ErrorCode == "ACCOUNT_401":
				fmt.Printf("   • %s: 凭证过期，请更新 %s 的登录凭证\n", r.Platform, r.Platform)
			case r.ErrorCode == "NETWORK_UNREACHABLE":
				fmt.Printf("   • %s: 网络不可达，请检查 API 地址和网络连接\n", r.Platform)
			case r.ErrorCode == "API_TIMEOUT":
				fmt.Printf("   • %s: API 超时，请检查平台是否在线\n", r.Platform)
			case r.ErrorCode == "PLATFORM_RATE_LIMITED":
				fmt.Printf("   • %s: 被限流，请稍后再试\n", r.Platform)
			case r.ErrorCode == "CREDENTIAL_FAILED":
				fmt.Printf("   • %s: 凭证无效，请检查格式是否正确\n", r.Platform)
			case r.ErrorCode == "A1_UNAVAILABLE":
				fmt.Printf("   • %s: A1 凭证服务不可用\n", r.Platform)
			case r.ErrorCode == "DECRYPT_FAILED":
				fmt.Printf("   • %s: 凭证解密失败，可能是凭证数据损坏\n", r.Platform)
			case r.Status == "fail" && r.ErrorCode != "":
				fmt.Printf("   • %s: %s\n", r.Platform, r.ErrorMessage)
			}
		}
	}
}

func orNA(s string) string {
	if s == "" {
		return "(空)"
	}
	return s
}

func newMockA1Server(vault *a1_account.MockSecretVault) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/account/credentials" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var reqBody struct {
			AccountID string `json:"AccountID"`
			UID       string `json:"UID"`
			Caller    string `json:"Caller"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(c1_publisher.A1Error{Code: "INVALID_INPUT", Message: err.Error()})
			return
		}
		resp, err := vault.GetCredentials(r.Context(), a1_account.GetCredentialsRequest{
			AccountID: reqBody.AccountID,
			UID:       reqBody.UID,
			Caller:    reqBody.Caller,
		})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(c1_publisher.A1Error{Code: "UNKNOWN", Message: err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}
