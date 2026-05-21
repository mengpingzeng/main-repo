package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ai_provider/config"
	"ai_provider/api"
)

func main() {
	port := flag.Int("port", 18180, "HTTP server port")
	configPath := flag.String("config-path", "", "Path to sandbox opencode config to enrich (required, e.g. /tmp/sm_demo/opencode_config.json)")
	keysConfig := flag.String("keys-config", "config/keys.json", "Path to API keys JSON config file (supports multiple keys per provider for round-robin rotation)")
	flag.Parse()

	if *configPath == "" {
		log.Fatal("--config-path is required.\n" +
			"  This is the SANDBOX config path (managed by Session Manager),\n" +
			"  NOT the global ~/.config/opencode/opencode.json.\n" +
			"  Example: --config-path /tmp/sm_demo/opencode_config.json")
	}

	log.Printf("Sandbox config path: %s", *configPath)
	log.Printf("Keys config path: %s", *keysConfig)

	mgr, err := config.NewManager(*keysConfig)
	if err != nil {
		log.Fatalf("Failed to init AI Provider: %v", err)
	}

	// ② 将 provider 节补充到沙箱配置中
	if err := mgr.EnrichConfig(*configPath); err != nil {
		log.Fatalf("Failed to enrich sandbox config: %v", err)
	}
	log.Printf("Provider config written to: %s", *configPath)

	// ③ 打印加载的 provider 和模型
	for k, p := range mgr.GetProviderConfig().Provider {
		log.Printf("Provider [%s]: %d models, base_url=%s", k, len(p.Models), p.BaseURL)
	}
	log.Printf("Total available models: %d", len(mgr.ListModels()))
	for _, m := range mgr.ListModels() {
		log.Printf("  - %s (%s) → %s", m.ID, m.Provider, m.RecommendedFor)
	}

	// ④ 配置健康检查
	warnings := mgr.Validate()
	if len(warnings) > 0 {
		log.Println("Warnings:")
		for _, w := range warnings {
			log.Printf("  ⚠ %s", w)
		}
	} else {
		log.Println("Configuration validation: all OK")
	}

	// ⑤ 启动 HTTP 服务
	server := api.NewServer(mgr, *port)

	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	log.Printf("AI Provider ready. API at http://localhost:%d/api/models", *port)
	log.Printf("Health check at http://localhost:%d/healthz", *port)
	log.Printf("Config status at http://localhost:%d/api/config/status", *port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("AI Provider shutting down...")
}
