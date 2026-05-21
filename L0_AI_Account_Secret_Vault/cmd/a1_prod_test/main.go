package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ============================================================
// 测试参数
// ============================================================

const (
	testPlatform = "fanqie"

	publishScriptPath = "/home/claw_studios/code/L1_AI_Releaser/scripts/publish_fanqie.js"
)

var (
	testUsername string
	testPassword string
	testUID      string
	testToken    string
)

// testCookie 生产自测用 cookie，仅用于验证加解密链路。
// 安全约束：不在任何日志/输出中打印此值。
var testCookie = "csrf_session_id=756c1b62754eea02317b09b8ac770251; d_ticket=202dbb8ed0b6414ba500dcaf93356b42608e8; gfkadpd=2503,36144; has_biz_token=false; Hm_lpvt_2667d29c8e792e6fa9182c20a3013175=1778134285; Hm_lvt_2667d29c8e792e6fa9182c20a3013175=1778133278; HMACCOUNT=E64A46435BBD5440; is_staff_user=false; n_mh=5WFXd-UzHeyfFeaziGoadVZ7sm7mnnaxHdtBbRXEkh4; novel_web_id=7637024208249357870; odin_tt=bfe0eaac9159f4da8ab4cb6dc1494b8b625f34b1120c99981cef1c253d2500087658e31ae0b088c61c0218624379c9014b9120fc89e52df90ea712fc022b3e5c; passport_auth_status=1fb97ffe3b6477c6cfe92023b50433de%2C; passport_auth_status_ss=1fb97ffe3b6477c6cfe92023b50433de%2C; passport_csrf_token=5820ee7db69872f38a0a248dbae6e91a; passport_csrf_token_default=5820ee7db69872f38a0a248dbae6e91a; passport_mfa_token=CjfNQ0ByEPeDnd6trvkniiPeC9NwGN%2FoActN7s8M9TFYK6%2BHu6p29cyUr4lhaWySeCyPbvtKcAtGGkoKPAAAAAAAAAAAAABQZEH8PlXdUc%2FLYbs4TPZ76vTxIwIK%2FjlfNY7ufGMQFySfIL75qa37tlow8kYE5YMKsBCB55AOGPax0WwgAiIBA5urejY%3D; s_v_web_id=verify_mov2n6vg_6BTillnr_NEiQ_4FSy_B6Jp_Rykht5oXTK8u; serial_uuid=7637024208249357870; serial_webid=7637024208249357870; session_tlb_tags=ttt%7C19%7CWiORu08YW7g6EKYyygfnlf_________3kBaEKxH1mghfd2bfFlKy71gGb1rXpH5ChFOCFTRQn0s%3D; sessionid=5a2391bb4f185bb83a10a632ca07e795; sessionid_ss=5a2391bb4f185bb83a10a632ca07e795; sid_guard=5a2391bb4f185bb83a10a632ca07e795%7C1778133308%7C5184000%7CMon%2C+06-Jul-2026+05%3A55%3A08+GMT; sid_tt=5a2391bb4f185bb83a10a632ca07e795; sid_ucp_v1=1.0.0-KGFiNmNhZmUyNGI2OWVjMTMyZmJhZjJiNjE2MzI1NjZiZjNmNmFhMTIKHwjbiLC6742WBxC80vDPBhjHEyAMMLTqvY8GOAJA8QcaAmxxIiA1YTIzOTFiYjRmMTg1YmI4M2ExMGE2MzJjYTA3ZTc5NQ; ssid_ucp_v1=1.0.0-KGFiNmNhZmUyNGI2OWVjMTMyZmJhZjJiNjE2MzI1NjZiZjNmNmFhMTIKHwjbiLC6742WBxC80vDPBhjHEyAMMLTqvY8GOAJA8QcaAmxxIiA1YTIzOTFiYjRmMTg1YmI4M2ExMGE2MzJjYTA3ZTc5NQ; ttwid=1%7C3-PLRg3pqSMacLujnp0bAkR2xtNXWH3nPw8B08LIji8%7C1778134521%7C9812815fbc71ed7c1ba8fa67ad6ef4b0ceef6a703c1eb923c252c68de55a3d79; uid_tt=fbafa4108a0bf9c97a15041de1e1e56e; uid_tt_ss=fbafa4108a0";// ============================================================
// HTTP 请求/响应结构体
// ============================================================

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authResponse struct {
	UID      string `json:"uid"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

type bindRequest struct {
	UID                  string `json:"uid,omitempty"`
	Platform             string `json:"platform"`
	CredentialsPlaintext string `json:"credentials_plaintext"`
	Caller               string `json:"caller"`
}

type bindResponse struct {
	AccountID     string `json:"account_id"`
	UID           string `json:"uid"`
	Platform      string `json:"platform"`
	MaskedDisplay string `json:"masked_display"`
	IsNewBinding  bool   `json:"is_new_binding"`
	BoundAt       string `json:"bound_at"`
}

type unbindRequest struct {
	AccountID string `json:"account_id"`
	UID       string `json:"uid,omitempty"`
	Caller    string `json:"caller"`
}

type unbindResponse struct {
	AccountID string `json:"account_id"`
	UnboundAt string `json:"unbound_at"`
}

type getCredentialsRequest struct {
	AccountID string `json:"account_id"`
	UID       string `json:"uid"`
	Caller    string `json:"caller"`
}

type getCredentialsResponse struct {
	AccountID       string `json:"account_id"`
	UID             string `json:"uid"`
	Platform        string `json:"platform"`
	Credentials     string `json:"credentials"`
	MaskedDisplay   string `json:"masked_display"`
	SecurityWarning string `json:"security_warning"`
}

type listResponse struct {
	Accounts []accountSummary `json:"accounts"`
	Total    int              `json:"total"`
}

type accountSummary struct {
	AccountID     string `json:"account_id"`
	UID           string `json:"uid"`
	Platform      string `json:"platform"`
	MaskedDisplay string `json:"masked_display"`
	BoundAt       string `json:"bound_at"`
	UpdatedAt     string `json:"updated_at"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ============================================================
// 辅助函数
// ============================================================

func printResult(testNum int, name string, passed bool, detail string) {
	status := "PASS"
	if !passed {
		status = "FAIL"
	}
	fmt.Printf("  [%s] Test %d: %s", status, testNum, name)
	if detail != "" {
		fmt.Printf(" — %s", detail)
	}
	fmt.Println()
}

func httpPost(url string, body interface{}) (*http.Response, []byte, error) {
	return httpPostWithAuth(url, body, "")
}

func httpPostWithAuth(url string, body interface{}, token string) (*http.Response, []byte, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return resp, data, err
}

func httpGet(url string) (*http.Response, []byte, error) {
	return httpGetWithAuth(url, "")
}

func httpGetWithAuth(url string, token string) (*http.Response, []byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return resp, data, err
}

func generateTestContent() string {
	paragraphs := []string{
		"这是一篇用于 A1 Secret Vault 生产环境自测的测试章节。",
		"本测试验证完整的凭证加密存储与解密链路：从 BFF 绑定 cookie，到 A1 加密存储，",
		"再到 C1 发布模块通过 GetCredentials 获取解密凭证并调用发布脚本。",
		"",
		"第一章　测试开始",
		"",
		"那天清晨，阳光透过薄雾洒在窗台上，空气中弥漫着泥土和青草的气息。",
		"我坐在书桌前，翻开笔记本的第一页，开始记录这次测试的每一个细节。",
		"加密算法的精妙之处在于，它让一段看似无意义的密文，在正确的密钥面前，",
		"重新变回原来的模样——就像记忆，需要一个特定的触点才能被唤醒。",
		"",
		"我合上笔记本，走到窗前。远处的山峦在晨光中若隐若现，如同一幅水墨画。",
		"这次测试的链路虽然不长，但每一个环节都至关重要：绑定的那一刻，",
		"cookie 被 AES-256-GCM 加密，变成了一串无法辨认的字节；",
		"而在 C1 调用 GetCredentials 的那一刻，同样的密钥让密文重现原形。",
		"这是一次完美的对称加密循环，也是对系统安全性的一次全面检验。",
		"",
		"我重新打开笔记本，写下了最后一句话：",
		"「安全不是一道防线，而是一种习惯。每一次加密，都是对用户数据的承诺。」",
		"",
		"（全文完）",
		"",
		"注：本章节由 A1 Secret Vault 生产自测脚本自动生成，仅用于验证完整加解密链路。",
	}
	content := strings.Join(paragraphs, "\n")
	for len([]rune(content)) < 1050 {
		content += "\n补充测试内容以确保满足最低字数要求。"
	}
	return content
}

func authSetup(baseURL string) bool {
	testUsername = fmt.Sprintf("test_user_%d", time.Now().Unix())
	testPassword = "test_password_123"

	resp, data, err := httpPost(baseURL+"/api/auth/register", registerRequest{
		Username: testUsername,
		Password: testPassword,
	})
	if err != nil {
		fmt.Printf("  注册请求失败: %v\n  (回退至登录流程)\n", err)
	} else if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		fmt.Printf("  注册失败 HTTP %d: %s\n  (回退至登录流程)\n", resp.StatusCode, string(data))
	}

	resp, data, err = httpPost(baseURL+"/api/auth/login", loginRequest{
		Username: testUsername,
		Password: testPassword,
	})
	if err != nil {
		fmt.Printf("  登录失败: %v\n", err)
		return false
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("  登录失败 HTTP %d: %s\n", resp.StatusCode, string(data))
		return false
	}

	var ar authResponse
	if err := json.Unmarshal(data, &ar); err != nil {
		fmt.Printf("  解析登录响应失败: %v\n", err)
		return false
	}

	testUID = ar.UID
	testToken = ar.Token
	fmt.Printf("  登录成功 uid=%s username=%s\n", testUID, testUsername)
	return true
}

// ============================================================
// 测试用例
// ============================================================

func test1_Bind(baseURL string) (string, bool) {
	resp, data, err := httpPostWithAuth(baseURL+"/api/account/bind", bindRequest{
		Platform:             testPlatform,
		CredentialsPlaintext: testCookie,
		Caller:               "bff",
	}, testToken)
	if err != nil {
		printResult(1, "Bind → IsNewBinding=true", false, fmt.Sprintf("请求失败: %v", err))
		return "", false
	}
	if resp.StatusCode != http.StatusOK {
		printResult(1, "Bind → IsNewBinding=true", false, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(data)))
		return "", false
	}

	var br bindResponse
	if err := json.Unmarshal(data, &br); err != nil {
		printResult(1, "Bind → IsNewBinding=true", false, fmt.Sprintf("JSON 解析失败: %v", err))
		return "", false
	}

	if br.AccountID == "" {
		printResult(1, "Bind → IsNewBinding=true", false, "AccountID 为空")
		return "", false
	}
	if !br.IsNewBinding {
		printResult(1, "Bind → IsNewBinding=true", false, "IsNewBinding=false (期望 true)")
		return "", false
	}

	printResult(1, "Bind → IsNewBinding=true", true,
		fmt.Sprintf("AccountID=%s IsNewBinding=true", br.AccountID))
	return br.AccountID, true
}

func test2_ListContainsAccount(baseURL string, accountID string) bool {
	resp, data, err := httpGetWithAuth(baseURL+"/api/account/list", testToken)
	if err != nil {
		printResult(2, "List → 含已绑定账号", false, fmt.Sprintf("请求失败: %v", err))
		return false
	}
	if resp.StatusCode != http.StatusOK {
		printResult(2, "List → 含已绑定账号", false, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(data)))
		return false
	}

	var lr listResponse
	if err := json.Unmarshal(data, &lr); err != nil {
		printResult(2, "List → 含已绑定账号", false, fmt.Sprintf("JSON 解析失败: %v", err))
		return false
	}

	for _, acc := range lr.Accounts {
		if acc.AccountID == accountID {
			printResult(2, "List → 含已绑定账号", true,
				fmt.Sprintf("找到 AccountID=%s Total=%d", accountID, lr.Total))
			return true
		}
	}
	printResult(2, "List → 含已绑定账号", false, "列表中未找到已绑定账号")
	return false
}

func test3_GetCredentials(baseURL string, accountID string) bool {
	resp, data, err := httpPost(baseURL+"/api/account/credentials", getCredentialsRequest{
		AccountID: accountID,
		UID:       testUID,
		Caller:    "c1_publisher",
	})
	if err != nil {
		printResult(3, "GetCredentials → cookie + SecurityWarning", false, fmt.Sprintf("请求失败: %v", err))
		return false
	}
	if resp.StatusCode != http.StatusOK {
		printResult(3, "GetCredentials → cookie + SecurityWarning", false, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(data)))
		return false
	}

	var gcr getCredentialsResponse
	if err := json.Unmarshal(data, &gcr); err != nil {
		printResult(3, "GetCredentials → cookie + SecurityWarning", false, fmt.Sprintf("JSON 解析失败: %v", err))
		return false
	}

	if gcr.Credentials == "" {
		printResult(3, "GetCredentials → cookie + SecurityWarning", false, "Credentials 为空")
		return false
	}
	if gcr.SecurityWarning != "SENSITIVE: DO NOT LOG" {
		printResult(3, "GetCredentials → cookie + SecurityWarning", false, "SecurityWarning 缺失/异常")
		return false
	}

	printResult(3, "GetCredentials → cookie + SecurityWarning", true,
		fmt.Sprintf("Credentials 长度=%d SecurityWarning 存在", len(gcr.Credentials)))
	return true
}

func test4_GetCredentialsWrongCaller(baseURL string, accountID string) bool {
	resp, data, err := httpPost(baseURL+"/api/account/credentials", getCredentialsRequest{
		AccountID: accountID,
		UID:       testUID,
		Caller:    "bff",
	})
	if err != nil {
		printResult(4, "非 c1_publisher → 403", false, fmt.Sprintf("请求失败: %v", err))
		return false
	}
	if resp.StatusCode != http.StatusForbidden {
		printResult(4, "非 c1_publisher → 403", false, fmt.Sprintf("HTTP %d (期望 403): %s", resp.StatusCode, string(data)))
		return false
	}

	var er errorResponse
	json.Unmarshal(data, &er)
	printResult(4, "非 c1_publisher → 403", true, fmt.Sprintf("HTTP 403 Code=%s", er.Code))
	return true
}

func test5_UIDMismatch(baseURL string, accountID string) bool {
	resp, data, err := httpPost(baseURL+"/api/account/credentials", getCredentialsRequest{
		AccountID: accountID,
		UID:       "attacker_uid",
		Caller:    "c1_publisher",
	})
	if err != nil {
		printResult(5, "UID 不匹配 → 403", false, fmt.Sprintf("请求失败: %v", err))
		return false
	}
	if resp.StatusCode != http.StatusForbidden {
		printResult(5, "UID 不匹配 → 403", false, fmt.Sprintf("HTTP %d (期望 403): %s", resp.StatusCode, string(data)))
		return false
	}

	var er errorResponse
	json.Unmarshal(data, &er)
	printResult(5, "UID 不匹配 → 403", true, fmt.Sprintf("HTTP 403 Code=%s", er.Code))
	return true
}

func test6_UnbindThenCredentials404(baseURL string, accountID string) bool {
	resp, data, err := httpPostWithAuth(baseURL+"/api/account/unbind", unbindRequest{
		AccountID: accountID,
		Caller:    "bff",
	}, testToken)
	if err != nil {
		printResult(6, "解绑后 GetCredentials → 404", false, fmt.Sprintf("Unbind 请求失败: %v", err))
		return false
	}
	if resp.StatusCode != http.StatusOK {
		printResult(6, "解绑后 GetCredentials → 404", false, fmt.Sprintf("Unbind HTTP %d: %s", resp.StatusCode, string(data)))
		return false
	}

	time.Sleep(200 * time.Millisecond)

	resp, data, err = httpPost(baseURL+"/api/account/credentials", getCredentialsRequest{
		AccountID: accountID,
		UID:       testUID,
		Caller:    "c1_publisher",
	})
	if err != nil {
		printResult(6, "解绑后 GetCredentials → 404", false, fmt.Sprintf("GetCredentials 请求失败: %v", err))
		return false
	}
	if resp.StatusCode == http.StatusNotFound {
		printResult(6, "解绑后 GetCredentials → 404", true, "HTTP 404 正确拒绝")
		return true
	}
	printResult(6, "解绑后 GetCredentials → 404", false, fmt.Sprintf("HTTP %d (期望 404): %s", resp.StatusCode, string(data)))
	return false
}

func test7_RebindDecryptable(baseURL string) (string, bool) {
	resp, data, err := httpPostWithAuth(baseURL+"/api/account/bind", bindRequest{
		Platform:             testPlatform,
		CredentialsPlaintext: testCookie,
		Caller:               "bff",
	}, testToken)
	if err != nil {
		printResult(7, "重新 Bind → 可获取凭证", false, fmt.Sprintf("Bind 请求失败: %v", err))
		return "", false
	}
	if resp.StatusCode != http.StatusOK {
		printResult(7, "重新 Bind → 可获取凭证", false, fmt.Sprintf("Bind HTTP %d: %s", resp.StatusCode, string(data)))
		return "", false
	}

	var br bindResponse
	if err := json.Unmarshal(data, &br); err != nil {
		printResult(7, "重新 Bind → 可获取凭证", false, fmt.Sprintf("JSON 解析失败: %v", err))
		return "", false
	}

	resp, data, err = httpPost(baseURL+"/api/account/credentials", getCredentialsRequest{
		AccountID: br.AccountID,
		UID:       testUID,
		Caller:    "c1_publisher",
	})
	if err != nil {
		printResult(7, "重新 Bind → 可获取凭证", false, fmt.Sprintf("GetCredentials 请求失败: %v", err))
		return br.AccountID, false
	}

	var gcr getCredentialsResponse
	if err := json.Unmarshal(data, &gcr); err != nil {
		printResult(7, "重新 Bind → 可获取凭证", false, fmt.Sprintf("JSON 解析失败: %v", err))
		return br.AccountID, false
	}

	if gcr.Credentials == "" {
		printResult(7, "重新 Bind → 可获取凭证", false, "Credentials 为空")
		return br.AccountID, false
	}
	if br.IsNewBinding {
		printResult(7, "重新 Bind → 可获取凭证", true,
			fmt.Sprintf("Credentials长度=%d IsNewBinding=true", len(gcr.Credentials)))
	} else {
		printResult(7, "重新 Bind → 可获取凭证", true,
			fmt.Sprintf("Credentials长度=%d (覆盖更新)", len(gcr.Credentials)))
	}
	return br.AccountID, true
}

func test8_PublishWithDecryptedCookie(baseURL string, accountID string) bool {
	resp, data, err := httpPost(baseURL+"/api/account/credentials", getCredentialsRequest{
		AccountID: accountID,
		UID:       testUID,
		Caller:    "c1_publisher",
	})
	if err != nil {
		printResult(8, "publish_fanqie.js 连通性", false, fmt.Sprintf("GetCredentials 请求失败: %v", err))
		return false
	}
	if resp.StatusCode != http.StatusOK {
		printResult(8, "publish_fanqie.js 连通性", false, fmt.Sprintf("GetCredentials HTTP %d: %s", resp.StatusCode, string(data)))
		return false
	}

	var gcr getCredentialsResponse
	if err := json.Unmarshal(data, &gcr); err != nil {
		printResult(8, "publish_fanqie.js 连通性", false, fmt.Sprintf("JSON 解析失败: %v", err))
		return false
	}

	if _, err := os.Stat(publishScriptPath); os.IsNotExist(err) {
		printResult(8, "publish_fanqie.js 连通性", false, fmt.Sprintf("脚本不存在: %s", publishScriptPath))
		return false
	}

	testTitle := fmt.Sprintf("A1_ProdTest_%s", time.Now().Format("150405"))
	testContent := generateTestContent()
	testNovelName := "A1_Prod_Test_Novel"

	input := map[string]interface{}{
		"title":         testTitle,
		"content":       testContent,
		"novelName":     testNovelName,
		"chapterNumber": 1,
	}
	jsonBytes, _ := json.Marshal(input)

	cmd := exec.Command("node", publishScriptPath, string(jsonBytes))
	cmd.Env = append(os.Environ(), "FANQIE_COOKIE="+gcr.Credentials)

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	cmd.Run()

	stderrOutput := stderrBuf.String()
	if strings.Contains(stderrOutput, "login confirmed") {
		printResult(8, "publish_fanqie.js 连通性", true, "login confirmed")
		return true
	}

	printResult(8, "publish_fanqie.js 连通性", false, "stderr 中未检测到 login confirmed（cookie 可能已过期）")
	return false
}

// ============================================================
// 主流程
// ============================================================

func main() {
	fmt.Println("========================================")
	fmt.Println(" A1 Secret Vault · 生产环境自测 (HTTP)")
	fmt.Println("========================================")
	fmt.Println()

	baseURL := os.Getenv("A1_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8084"
	}

	fmt.Printf("服务地址: %s\n", baseURL)
	fmt.Printf("Cookie 长度: %d 字符\n", len(testCookie))
	fmt.Println()

	fmt.Println("=== 用户认证 ===")
	if !authSetup(baseURL) {
		fmt.Println("认证失败，退出测试")
		os.Exit(1)
	}
	fmt.Println()

	// 先解绑清理旧数据（幂等）
	fmt.Println("=== 清理旧数据 ===")
	httpPostWithAuth(baseURL+"/api/account/unbind", unbindRequest{
		AccountID: "acc_user_001_fanqie",
		Caller:    "bff",
	}, testToken)
	time.Sleep(200 * time.Millisecond)
	fmt.Println("  清理完成")
	fmt.Println()

	// 运行测试
	fmt.Println("=== 测试开始 ===")
	fmt.Println()

	results := make([]bool, 9)
	var accountID string

	accountID, results[1] = test1_Bind(baseURL)
	results[2] = test2_ListContainsAccount(baseURL, accountID)
	results[3] = test3_GetCredentials(baseURL, accountID)
	results[4] = test4_GetCredentialsWrongCaller(baseURL, accountID)
	results[5] = test5_UIDMismatch(baseURL, accountID)
	results[6] = test6_UnbindThenCredentials404(baseURL, accountID)
	accountID, results[7] = test7_RebindDecryptable(baseURL)
	results[8] = test8_PublishWithDecryptedCookie(baseURL, accountID)

	fmt.Println()
	fmt.Println("=== 测试结果汇总 ===")

	passed := 0
	failed := 0
	for i := 1; i <= 8; i++ {
		if results[i] {
			passed++
		} else {
			failed++
		}
	}
	fmt.Printf("  通过: %d / 8\n", passed)
	fmt.Printf("  失败: %d / 8\n", failed)

	// 清理
	fmt.Println()
	fmt.Println("=== 清理 ===")
	httpPostWithAuth(baseURL+"/api/account/unbind", unbindRequest{
		AccountID: accountID,
		Caller:    "bff",
	}, testToken)
	fmt.Println("  测试账号已解绑")

	fmt.Println()
	if failed > 0 {
		fmt.Printf("存在 %d 个失败用例\n", failed)
		os.Exit(1)
	} else {
		fmt.Println("全部测试通过")
	}
}
