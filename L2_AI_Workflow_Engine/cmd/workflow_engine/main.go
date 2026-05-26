package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"L2_AI_Workflow_Engine"
	"clawstudios/pkg/logging"
	c1 "clawstudios/l1_ai_releaser/services/c1_publisher"
)

func main() {
	dsn := getEnv("WF_DB_DSN", "xlongxia:Xlongxia_123@tcp(127.0.0.1:3306)/xlongxia?parseTime=true")
	port := getEnv("WF_LISTEN_PORT", "9100")

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)

	wsHub := workflow_engine.NewWSHub()

	var c1Publisher workflow_engine.C1Publisher
	var mdWriter workflow_engine.MDWriter
	var draftFetcher workflow_engine.DraftFetcher = &workflow_engine.SMDraftFetcher{
		DataDir: getEnv("SM_DATA_DIR", "/tmp/sm_demo"),
	}

	a4StorageDir := getEnv("A4_STORAGE_DIR", "/tmp/sm_demo")
	a4Adapter, err := workflow_engine.NewRealMDWriterAdapter(a4StorageDir)
	if err != nil {
		log.Fatalf("init A4 adapter (storage=%s): %v", a4StorageDir, err)
	}
	mdWriter = a4Adapter
	log.Printf("using RealMDWriter (A4 storage=%s)", a4StorageDir)

	a1BaseURL := getEnv("A1_BASE_URL", "http://localhost:8084")
	fanqieAdapter := c1.NewFanqiePublishAdapter(c1.AdapterConfig{
		ScriptPath: getEnv("FANQIE_SCRIPT", "/home/claw_studios/code/L1_AI_Releaser/scripts/publish_fanqie.js"),
		Timeout:    300 * time.Second,
	})
	zhulangAdapter := c1.NewZhulangPublishAdapter(c1.AdapterConfig{
		ScriptPath: getEnv("ZHULANG_SCRIPT", "/home/claw_studios/code/L1_AI_Releaser/scripts/publish_zhulang.js"),
		Timeout:    300 * time.Second,
	})
	realPub := c1.NewRealPublisher(c1.Config{
		A1BaseURL: a1BaseURL,
		Adapters:  []c1.PublishAdapter{fanqieAdapter, zhulangAdapter},
		DB:        db,
	})
	adapter := workflow_engine.NewRealC1Adapter(realPub, "", "", "")
	c1Publisher = adapter
	log.Printf("using RealC1Publisher (A1=%s)", a1BaseURL)

	eng := workflow_engine.New(workflow_engine.EngineConfig{
		DB:           db,
		C1Publisher:  c1Publisher,
		MDWriter:     mdWriter,
		DraftFetcher: draftFetcher,
		WSHub:        wsHub,
	})

	// 启动时恢复未完成任务
	go func() {
		if err := eng.RecoverAll(context.Background()); err != nil {
			log.Printf("recover err: %v", err)
		}
	}()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/task/", func(w http.ResponseWriter, r *http.Request) {
		logger := logging.FromContext(r.Context())
		// 简化路由：POST /api/task/{tid}/publish
		if r.Method == http.MethodPost && len(r.URL.Path) > 15 && r.URL.Path[len(r.URL.Path)-8:] == "/publish" {
			var input workflow_engine.PublishInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				if logger != nil {
					logger.Error(logging.ErrInvalidParam, "decode publish input failed: %v", err)
				}
				http.Error(w, `{"errorCode":"BAD_REQUEST"}`, 400)
				return
			}
			task, err := eng.ExecuteAndGetTask(r.Context(), input)
			if err != nil {
				if logger != nil {
					logger.Error(logging.ErrWorkflowError, "ExecuteAndGetTask failed: %v", err)
				}
				w.WriteHeader(500)
				json.NewEncoder(w).Encode(map[string]string{
					"errorCode": "INTERNAL_ERROR", "errorMessage": err.Error(),
				})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":   task.Status,
				"taskId":   task.TaskID,
				"results":  task.PublishResults,
			})
			return
		}
		// POST /api/task/{tid}/replay
		if r.Method == http.MethodPost && len(r.URL.Path) > 14 && r.URL.Path[len(r.URL.Path)-7:] == "/replay" {
			taskID := extractTaskID(r.URL.Path)
			if err := eng.Replay(r.Context(), taskID); err != nil {
				if logger != nil {
					logger.Error(logging.ErrWorkflowError, "Replay(%s) failed: %v", taskID, err)
				}
				w.WriteHeader(500)
				json.NewEncoder(w).Encode(map[string]string{
					"errorCode": "INTERNAL_ERROR", "errorMessage": err.Error(),
				})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "replaying"})
			return
		}
		// GET /api/task/{tid}/status
		if r.Method == http.MethodGet && len(r.URL.Path) > 15 && r.URL.Path[len(r.URL.Path)-7:] == "/status" {
			taskID := extractTaskID(r.URL.Path)
			task, err := eng.GetTask(r.Context(), taskID)
			if err != nil {
				if logger != nil {
					logger.Error(logging.ErrDatabaseError, "查询任务状态失败: task=%s err=%v", taskID, err)
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"task_id": taskID,
					"status":  "",
					"exists":  false,
				})
				return
			}
			if logger != nil {
				logger.Info("查询任务状态成功: task=%s status=%s session=%s chapter=%d",
					taskID, task.Status, task.SessionID, task.ChapterNumber)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"task_id":         task.TaskID,
				"status":          task.Status,
				"session_id":      task.SessionID,
				"volume_name":     task.VolumeName,
				"chapter_number":  task.ChapterNumber,
				"error_msg":       task.ErrorMsg,
				"publish_results": task.PublishResults,
				"accounts":        task.Accounts,
				"created_at":      task.CreatedAt,
				"updated_at":      task.UpdatedAt,
				"exists":          true,
			})
			return
		}
		http.Error(w, `{"errorCode":"NOT_FOUND"}`, 404)
	})

	mux.HandleFunc("/ws/task/", func(w http.ResponseWriter, r *http.Request) {
		taskID := extractTaskID(r.URL.Path)
		c := wsHub.Subscribe(taskID)
		defer wsHub.Unsubscribe(c)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		for data := range c.Ch {
			w.Write(data)
			flusher.Flush()
		}
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Printf("Workflow Engine listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, logging.HTTPMiddleware("WorkflowEngine")(mux)))
}

func extractTaskID(path string) string {
	parts := splitPath(path)
	for i, p := range parts {
		if p == "task" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func splitPath(path string) []string {
	var parts []string
	start := 1
	for i := 1; i < len(path); i++ {
		if path[i] == '/' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
