// cmd/mock_platform_server 模拟小红书/公众号的 API 服务器。
//
// 启动后，两个 HTTP 平台 API 分别在以下端口监听：
//
//	小红书:  http://localhost:9100
//	公众号:  http://localhost:9101
//
// 注：番茄小说使用 Puppeteer 浏览器自动化，无需本服务器模拟。
// 测试番茄小说请直接创建 FanqiePublishAdapter + 真实 cookie。
//
// 配合 real_demo 使用：
//
//	go run ./cmd/mock_platform_server/ &
//	ADAPTER_MODE=real go run ./cmd/real_demo/
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type platformServer struct {
	name    string
	port    string
	addr    string
	handler http.HandlerFunc
}

func main() {
	servers := []platformServer{
		{
			name: "小红书",
			port: envOr("MOCK_XHS_PORT", "9100"),
			addr: "",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handlePlatform(w, r, "xhs", "note", checkXHSCookie)
			},
		},
		{
			name: "公众号",
			port: envOr("MOCK_WECHAT_PORT", "9101"),
			addr: "",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handlePlatform(w, r, "wechat", "media", checkWechatToken)
			},
		},
		{
			name: "番茄小说",
			port: envOr("MOCK_FANQIE_PORT", "9102"),
			addr: "",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handlePlatform(w, r, "fanqie", "ch", checkFanqieToken)
			},
		},
	}

	fmt.Println("========================================")
	fmt.Println("  模拟平台 API 服务器")
	fmt.Println("========================================")
	fmt.Println()

	var started int
	for i := range servers {
		s := &servers[i]
		s.addr = ":" + s.port

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[%s] %s %s", s.name, r.Method, r.URL.Path)
			s.handler(w, r)
		})

		go func(srv *platformServer) {
			fmt.Printf("  🟢 %-10s → http://localhost%s\n", srv.name, srv.port)
			if err := http.ListenAndServe(srv.addr, mux); err != nil {
				log.Printf("[%s] 启动失败: %v", srv.name, err)
			}
		}(s)
		started++
	}

	fmt.Println()
	fmt.Println("  模拟特性:")
	fmt.Println("    正常请求           → 200 + mock post_id")
	fmt.Println("    cookie 'expired'   → 401（凭证过期）")
	fmt.Println("    cookie '429'       → 429（限流）")
	fmt.Println("    cookie 'slow'      → 延迟 5 秒（超时）")
	fmt.Println("    cookie '500'       → 500（服务器错误）")
	fmt.Println()
	fmt.Printf("  已启动 %d 个服务，按 Ctrl+C 退出\n\n", started)

	select {}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// --- 平台处理逻辑 ---

func handlePlatform(w http.ResponseWriter, r *http.Request, platform, prefix string,
	checkCred func(string) (int, string)) {

	cred := extractCredential(r, platform)

	statusCode, msg := checkCred(cred)
	if statusCode != 200 && statusCode != 0 {
		writeJSON(w, statusCode, map[string]interface{}{
			"code":    statusCode,
			"message": msg,
		})
		return
	}

	postID := fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano()%100000)

	writeJSON(w, 200, map[string]interface{}{
		"code": 0,
		"data": map[string]interface{}{postID: postID},
		"message": "ok",
	})
}

func extractCredential(r *http.Request, platform string) string {
	switch platform {
	case "xhs":
		return r.Header.Get("Cookie")
	case "wechat":
		// 从 URL query 中提取 access_token
		return r.URL.Query().Get("access_token")
	case "fanqie":
		auth := r.Header.Get("Authorization")
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func checkXHSCookie(cookie string) (int, string) {
	if cookie == "" {
		return 200, "" // Mock 模式下空 cookie 也通过
	}
	if strings.Contains(cookie, "expired") {
		return 401, "cookie expired"
	}
	if strings.Contains(cookie, "429") {
		return 429, "rate limited"
	}
	if strings.Contains(cookie, "slow") {
		time.Sleep(5 * time.Second)
	}
	if strings.Contains(cookie, "500") {
		return 500, "internal server error"
	}
	return 200, ""
}

func checkWechatToken(token string) (int, string) {
	if token == "" {
		return 200, ""
	}
	if strings.Contains(token, "expired") {
		return 401, "access_token expired"
	}
	if strings.Contains(token, "429") {
		return 429, "rate limited"
	}
	if strings.Contains(token, "slow") {
		time.Sleep(5 * time.Second)
	}
	if strings.Contains(token, "500") {
		return 500, "internal server error"
	}
	return 200, ""
}

func checkFanqieToken(token string) (int, string) {
	if token == "" {
		return 200, ""
	}
	if strings.Contains(token, "expired") {
		return 401, "access_token expired"
	}
	if strings.Contains(token, "429") {
		return 429, "rate limited"
	}
	if strings.Contains(token, "slow") {
		time.Sleep(5 * time.Second)
	}
	if strings.Contains(token, "500") {
		return 500, "internal server error"
	}
	return 200, ""
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
