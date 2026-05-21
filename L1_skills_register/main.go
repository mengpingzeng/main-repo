package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"L1_skills_register/api"
	"L1_skills_register/registry"
	"L1_skills_register/store"
)

func main() {
	port := flag.Int("port", 18090, "HTTP server port")
	ossDir := flag.String("oss-dir", "/tmp/skills_data", "OSS storage directory")
	internalAuth := flag.String("internal-auth", "", "Internal auth token (empty = no auth)")
	skillDir := flag.String("skill-dir", "fixtures", "Directory to scan for skill packages")
	flag.Parse()

	if err := os.MkdirAll(*ossDir, 0755); err != nil {
		log.Fatalf("Failed to create OSS dir: %v", err)
	}

	skillStore := store.NewFakeSkillStore()

	reg := registry.New(skillStore)

	absSkillDir, err := filepath.Abs(*skillDir)
	if err != nil {
		log.Fatalf("Failed to resolve skill-dir: %v", err)
	}
	fmt.Printf("Scanning skill directory: %s\n", absSkillDir)
	summary, err := reg.LoadFromDirectory(context.Background(), absSkillDir)
	if err != nil {
		fmt.Printf("  Scan error: %v\n", err)
	} else {
		fmt.Printf("  Total found: %d, Registered: %d, Skipped: %d, Errors: %d\n",
			summary.Total, summary.Registered, summary.Skipped, summary.Errors)
		for _, r := range summary.Results {
			status := "registered"
			if r.Skipped {
				status = "skipped"
			}
			if r.Error != "" {
				status = fmt.Sprintf("error: %s", r.Error)
			}
			fmt.Printf("    [%s] %s\n", status, r.Dir)
		}
	}

	handler := api.NewHandler(reg, *internalAuth)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	wrappedMux := api.LoggingMiddleware(api.CORSMiddleware(mux))

	fmt.Printf("Skill Registry starting\n")
	fmt.Printf("  Storage:    memory (fake store)\n")
	fmt.Printf("  OSS:        local://%s\n", *ossDir)
	fmt.Printf("  Skill dir:  %s\n", absSkillDir)
	fmt.Printf("  API port:   %d\n", *port)
	fmt.Printf("HTTP server listening on :%d\n", *port)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), wrappedMux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
