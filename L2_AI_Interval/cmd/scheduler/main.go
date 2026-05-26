package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/robfig/cron/v3"

	"claw_studios/L2_AI_Interval/internal/adapter"
	"claw_studios/L2_AI_Interval/internal/config"
	"claw_studios/L2_AI_Interval/internal/cycle"
	"claw_studios/L2_AI_Interval/internal/health"
	"claw_studios/L2_AI_Interval/internal/metrics"
	"clawstudios/pkg/logging"
)

func main() {
	cfg, err := config.LoadConfig(os.Getenv("SCHEDULER_CONFIG_PATH"))
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := sql.Open("mysql", cfg.Database.DSN)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	var statsAdapter adapter.StatsAdapter
	if os.Getenv("SCHEDULER_USE_MOCK") == "true" {
		statsAdapter = adapter.NewMockAdapter()
		log.Println("[scheduler] using MockStatsAdapter")
	} else {
		statsAdapter = adapter.NewC1StatsAdapter(nil)
		log.Println("[scheduler] using C1StatsAdapter (direct injection)")
	}

	runner := cycle.NewRunner(db, statsAdapter, cycle.Config{
		BatchSize:     cfg.Scheduler.BatchSize,
		FetchTimeout:  cfg.Scheduler.FetchTimeout,
		BatchInterval: cfg.Scheduler.BatchInterval,
		MaxRetry:      cfg.Scheduler.MaxRetry,
		RetryBackoff:  cfg.Scheduler.RetryBackoff,
	})

	metrics.Register()

	shutdownCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := cron.New()
	c.AddFunc(cfg.Scheduler.CronExpr, func() {
		log.Println("[scheduler] cycle started")
		if err := runner.Run(shutdownCtx); err != nil {
			log.Printf("[scheduler] cycle error: %v", err)
		}
	})
	c.Start()
	log.Printf("[scheduler] cron started, expr=%s", cfg.Scheduler.CronExpr)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health.Handler(db))
	mux.HandleFunc("/metrics", metrics.Handler())
	addr := fmt.Sprintf(":%d", cfg.Scheduler.ListenPort)
	go func() {
		log.Printf("[scheduler] http listening on %s", addr)
		if err := http.ListenAndServe(addr, logging.HTTPMiddleware("Interval")(mux)); err != nil {
			log.Printf("[scheduler] http server: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("[scheduler] shutting down")
	cancel()
	c.Stop()

	ctx, timeout := context.WithTimeout(context.Background(), 10*time.Second)
	defer timeout()
	<-ctx.Done()
}
