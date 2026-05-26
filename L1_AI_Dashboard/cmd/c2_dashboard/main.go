package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"

	"L1_AI_Dashboard"
	"clawstudios/pkg/logging"
)

func main() {
	dsn := getEnv("C2_DB_DSN", "xlongxia:Xlongxia_123@tcp(127.0.0.1:3306)/xlongxia?parseTime=true")
	port := getEnv("C2_LISTEN_PORT", "8083")

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)

	querier := c2_dashboard.New(db)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/dashboard/query", func(w http.ResponseWriter, r *http.Request) {
		logger := logging.FromContext(r.Context())
		if r.Method != http.MethodPost {
			http.Error(w, `{"errorCode":"METHOD_NOT_ALLOWED"}`, 405)
			return
		}
		var req c2_dashboard.DashboardQueryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if logger != nil {
				logger.Error(logging.ErrInvalidParam, "decode query request failed: %v", err)
			}
			http.Error(w, `{"errorCode":"BAD_REQUEST"}`, 400)
			return
		}
		resp, err := querier.Query(r.Context(), req)
		if err != nil {
			if logger != nil {
				logger.Error(logging.ErrDatabaseError, "dashboard query failed: %v", err)
			}
			code, msg := c2_dashboard.ClassifyError(err)
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(map[string]string{
				"errorCode":    msg,
				"errorMessage": err.Error(),
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		logger := logging.FromContext(r.Context())
		if err := querier.Health(r.Context()); err != nil {
			if logger != nil {
				logger.Error(logging.ErrDatabaseError, "health check failed: %v", err)
			}
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Printf("C2 dashboard listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, logging.HTTPMiddleware("Dashboard")(mux)))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
