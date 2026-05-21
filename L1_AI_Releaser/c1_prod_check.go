package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"clawstudios/l1_ai_releaser/services/c1_publisher"

	_ "github.com/go-sql-driver/mysql"
)

const (
	testUID       = "user_001"
	testAccountID = "acc_653739aa471d"
	testPlatform  = "fanqie"
	testNovelName = "凌厉的觉醒"
	testTitle     = "初露锋芒"

	publishScriptPath = "/home/claw_studios/code/L1_AI_Releaser/scripts/publish_fanqie.js"
)

var testContent = `金色光芒散去之后，林越站在原地，体内那股狂暴的力量仍在翻涌。他低头看着自己的双手，掌心隐约有细密的金色纹路闪烁，像是某种古老的符文。
“哥……”林小荷的声音将他拉回现实。他转身，将妹妹护在身后。
陈天雄已经被护院们扶起，脸上写满了惊骇与愤怒。“你……你是修炼者？”陈天雄的声音有些发颤，“不可能，一个破村子里的药农，怎么可能是修炼者！”
林越没有回答。他也在问自己这个问题——刚才那股力量，从何而来？但他知道，现在不是探究的时候。
“小荷说的聚灵草，是在后山哪片区域找到的？”林越平静地问道，声音不大，却带着一种不容置疑的威压。
陈天雄脸色铁青。聚灵草的来历他心知肚明——那株灵草确实不是陈家的，是他手下人在后山偶然发现，便强占为己有。但凡稍有根基的家族，都不会把一株聚灵草放在眼里，可陈家不过是镇上的小门小户，三百灵石已是伤筋动骨。
“你陈家花了三百灵石买的？”林越嘴角勾起一抹冷笑，“那好。三天之内，带上灵石来后山找我。若敢再来骚扰我妹妹，我让你陈家从此在青石镇除名。”
话音落下，他抓起林小荷的手，转身离去。陈天雄望着那个瘦削的背影，嘴唇翥动了几下，终究没敢阻拦。刚才那股力量实在太骇人，那股威压，即便是镇上修为最高的吴长老也远远不及。
林越能感受到身后那些怨毒的目光，但他并不在意。他更在意的，是体内那股力量正在以一种诡异的速度消退，仿佛从未出现过。走出百丈之后，他的双腿开始发软，额头渗出细密的汗珠。
“哥，你没事吧？”林小荷察觉到了他的异样。
“没事。”林越咬牙，“先回家。”
回到那间破旧的木屋，他将林小荷安顿好，独自走进后山。月光如水，洒落在山涧之间。林越盘膝坐在一块青石上，闭目内视。
令他惊讶的是，他竟然能清晰地“看到”自己体内的状况——经脉之中，有一股微弱的金色气息在缓慢流动，仿佛一条沉睡的龙。
他试着引导那股气息，按照梦中的某种本能去运转。起初很艰难，金色气息纹丝不动。但他没有放弃，一遍又一遍地尝试。
不知过了多久，金色气息终于有了反应。它沿着某个奇异的路线缓慢流动，每经过一处穴位，那里的经脉便会被拓宽一丝。剧痛如同潮水般袭来，林越咬紧牙关，额头的青筋暴起。
他知道，这是经脉重塑的过程。记忆中，那似乎叫“开脉”。
时间一分一秒流逝，当黎明前的黑暗笼罩大地时，林越猛地睁开双眼。两道金光从他的瞳孔中射出，击穿了对面三丈外的巨石。碎石飞溅，惊起林间无数飞鸟。
他站起身，握紧拳头。原本干瘦的手臂青筋缠绕，肌肉线条如同鬼斧神工。他能感受到，自己此刻的力量，比凡人极限还要强出十倍不止。
不只是力量。他的感知范围扩大了数十倍，方圆千丈之内的一切都在他的感知之中——落叶坠地的轨迹、地底虫蚁的爬行、空气中灵气的走向……
一切都在告诉他同一个事实：这个世界，远比他曾经以为的要广阔得多。
林越抬头望向天际，云层之上，隐约可见一处巨大的阴影横亘在苍穹深处。那里，便是梦中所见的“九天”之外吗？
他不知道答案。但体内的金色气息在告诉他——真正的觉醒，才刚刚开始。`


func truncateForDisplay(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func printResult(testNum int, name string, passed bool, detail string) {
	icon := "\u2705"
	if !passed {
		icon = "\u274c"
	}
	fmt.Printf("  %s \u6d4b\u8bd5 %d\uff1a%s", icon, testNum, name)
	if detail != "" {
		fmt.Printf(" \u2014 %s", detail)
	}
	fmt.Println()
}

func test1_Platform() bool {
	adapter := c1_publisher.NewFanqiePublishAdapter(c1_publisher.AdapterConfig{
		ScriptPath: publishScriptPath,
	})
	if adapter.Platform() == "fanqie" {
		printResult(1, "Platform() = fanqie", true, "")
		return true
	}
	printResult(1, "Platform() = fanqie", false, fmt.Sprintf("got %s", adapter.Platform()))
	return false
}

func test2_CheckInput_EmptyText() bool {
	adapter := c1_publisher.NewFanqiePublishAdapter(c1_publisher.AdapterConfig{
		ScriptPath: publishScriptPath,
	})
	errMsg := adapter.CheckInput(c1_publisher.ProductContent{Title: "test", Text: ""})
	if errMsg != "" {
		printResult(2, "CheckInput empty text returns error", true, "")
		return true
	}
	printResult(2, "CheckInput empty text returns error", false, "no error returned")
	return false
}

func test3_CheckInput_ValidText() bool {
	adapter := c1_publisher.NewFanqiePublishAdapter(c1_publisher.AdapterConfig{
		ScriptPath: publishScriptPath,
	})
	errMsg := adapter.CheckInput(c1_publisher.ProductContent{
		Title: testTitle,
		Text:  testContent,
	})
	if errMsg == "" {
		printResult(3, "CheckInput >=1000 chars passes", true,
			fmt.Sprintf("text length=%d runes", len([]rune(testContent))))
		return true
	}
	printResult(3, "CheckInput >=1000 chars passes", false, errMsg)
	return false
}

func test4_Publish_EmptyCookie() bool {
	adapter := c1_publisher.NewFanqiePublishAdapter(c1_publisher.AdapterConfig{
		ScriptPath: publishScriptPath,
	})
	result := adapter.Publish(context.Background(), c1_publisher.ProductContent{
		Title: testTitle,
		Text:  testContent,
	}, "", "")
	if result.Status == "fail" && result.ErrorCode == c1_publisher.ErrCodeCredentialFailed {
		printResult(4, "Publish with empty cookie returns CREDENTIAL_FAILED", true, "")
		return true
	}
	printResult(4, "Publish with empty cookie returns error", false,
		fmt.Sprintf("status=%s code=%s", result.Status, result.ErrorCode))
	return false
}

func test5_GetRealCookie(ctx context.Context, a1URL string) (string, bool) {
	resp, err := c1_publisher.FetchCredential(ctx, a1URL, testAccountID, testUID)
	if err != nil {
		printResult(5, "GetCredentials from A1 HTTP", false,
			fmt.Sprintf("failed: %v", err))
		return "", false
	}
	if resp.Credentials == "" {
		printResult(5, "GetCredentials from A1 HTTP", false, "empty credential")
		return "", false
	}
	if resp.SecurityWarning != "SENSITIVE: DO NOT LOG" {
		printResult(5, "GetCredentials from A1 HTTP", false, "bad SecurityWarning")
		return "", false
	}
	printResult(5, "GetCredentials from A1 HTTP", true,
		fmt.Sprintf("cred len=%d has_sessionid=%v", len(resp.Credentials), strings.Contains(resp.Credentials, "sessionid")))
	return resp.Credentials, true
}

func test6_PublishChapter(ctx context.Context, a1URL string) bool {
	fanqie := c1_publisher.NewFanqiePublishAdapter(c1_publisher.AdapterConfig{
		ScriptPath: publishScriptPath,
		Timeout:    120 * time.Second,
	})
	publisher := c1_publisher.NewRealPublisher(c1_publisher.Config{
		A1BaseURL: a1URL,
		Adapters:  []c1_publisher.PublishAdapter{fanqie},
	})
	defer publisher.Close()

	
	req := c1_publisher.PublishRequest{
		TaskID: fmt.Sprintf("c1_prod_test_%d", time.Now().Unix()),
		Products: map[string]c1_publisher.ProductContent{
			testPlatform: {
				Title:         testTitle,
				Text:          testContent,
				NovelName:     testNovelName,
				VolumeName:    "auto_vol",
				
			},
		},
		Accounts: []c1_publisher.AccountRef{
			{AccountID: testAccountID, UID: testUID, Platform: testPlatform},
		},
		TraceID:   fmt.Sprintf("trace_c1_%d", time.Now().Unix()),
		SkillID:   "c1_prod_test_skill",
		SessionID: "c1_prod_test_session",
	}

	pubCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	resp, err := publisher.Publish(pubCtx, req)
	if err != nil {
		printResult(6, "Publish chapter via RealPublisher", false,
			fmt.Sprintf("error: %v", err))
		return false
	}
	if resp.Summary.Total != 1 {
		printResult(6, "Publish chapter via RealPublisher", false,
			fmt.Sprintf("Total=%d", resp.Summary.Total))
		return false
	}
	if resp.Summary.Succeeded != 1 {
		detail := ""
		for _, r := range resp.Results {
			if r.Status == "fail" {
				detail = fmt.Sprintf("%s: %s", r.ErrorCode, r.ErrorMessage)
			}
		}
		printResult(6, "Publish chapter via RealPublisher", false, detail)
		return false
	}
	var postID string
	for _, r := range resp.Results {
		if r.Status == "ok" {
			postID = r.PostID
		}
	}
	if postID == "" {
		printResult(6, "Publish chapter via RealPublisher", false, "ok but post_id empty")
		return false
	}
	printResult(6, "Publish chapter via RealPublisher", true,
		fmt.Sprintf("status=ok post_id=%s", truncateForDisplay(postID, 30)))
	return true
}

func test7_NoCredentialLeak(ctx context.Context, a1URL string) bool {
	fanqie := c1_publisher.NewFanqiePublishAdapter(c1_publisher.AdapterConfig{
		ScriptPath: publishScriptPath,
		Timeout:    120 * time.Second,
	})
	publisher := c1_publisher.NewRealPublisher(c1_publisher.Config{
		A1BaseURL: a1URL,
		Adapters:  []c1_publisher.PublishAdapter{fanqie},
	})
	defer publisher.Close()

	
	req := c1_publisher.PublishRequest{
		TaskID: fmt.Sprintf("c1_sec_test_%d", time.Now().Unix()),
		Products: map[string]c1_publisher.ProductContent{
			testPlatform: {
				Title:         testTitle,
				Text:          testContent,
				NovelName:     testNovelName,
				VolumeName:    "sec_vol",
				
			},
		},
		Accounts: []c1_publisher.AccountRef{
			{AccountID: testAccountID, UID: testUID, Platform: testPlatform},
		},
		TraceID:   fmt.Sprintf("trace_c1_sec_%d", time.Now().Unix()),
		SkillID:   "c1_sec_test_skill",
		SessionID: "c1_sec_test_session",
	}

	pubCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	resp, err := publisher.Publish(pubCtx, req)
	if err != nil {
		printResult(7, "PublishResult contains no cookie", false,
			fmt.Sprintf("error: %v", err))
		return false
	}

	sensitive := []string{"sessionid", "csrf_session_id", "passport_csrf_token", "d_ticket", "odin_tt", "sid_guard"}
	for _, r := range resp.Results {
		allFields := strings.Join([]string{r.AccountID, r.UID, r.Platform, r.PostID, r.ErrorCode, r.ErrorMessage}, "|")
		for _, sk := range sensitive {
			if strings.Contains(strings.ToLower(allFields), sk) {
				printResult(7, "PublishResult contains no cookie", false,
					fmt.Sprintf("leak of %s", sk))
				return false
			}
		}
	}
	printResult(7, "PublishResult contains no cookie", true, "all result fields clean")
	return true
}

func main() {
	fmt.Println("C1 Publisher / Production Test")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()

	passed, failed, skipped := 0, 0, 0

	fmt.Println("--- offline tests ---")
	fmt.Println()

	if test1_Platform() { passed++ } else { failed++ }
	if test2_CheckInput_EmptyText() { passed++ } else { failed++ }
	if test3_CheckInput_ValidText() { passed++ } else { failed++ }
	if test4_Publish_EmptyCookie() { passed++ } else { failed++ }

	fmt.Println()
	fmt.Println("--- integration tests (needs A1 HTTP service) ---")
	fmt.Println()

	a1URL := os.Getenv("A1_BASE_URL")
	if a1URL == "" {
		a1URL = "http://localhost:8084"
	}

	ctx := context.Background()
	_, ok5 := test5_GetRealCookie(ctx, a1URL)
	if ok5 {
		passed++
		if test6_PublishChapter(ctx, a1URL) { passed++ } else { failed++ }
		if test7_NoCredentialLeak(ctx, a1URL) { passed++ } else { failed++ }
	} else {
		failed++
		fmt.Println("  [SKIP] test 5 failed, skip 6-7")
		skipped += 2
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("  passed=%d  failed=%d  skipped=%d\n", passed, failed, skipped)
	fmt.Println(strings.Repeat("=", 50))

	fmt.Println()
	fmt.Println("security: no cookie in output; c1_prod_check.go not committed to git")

	if failed > 0 {
		os.Exit(1)
	}
}
