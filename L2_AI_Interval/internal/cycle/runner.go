package cycle

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"claw_studios/L2_AI_Interval/internal/adapter"
	"claw_studios/L2_AI_Interval/internal/metrics"
	"clawstudios/pkg/logging"
)

type Runner struct {
	db      *sql.DB
	adapter adapter.StatsAdapter
	cfg     Config
}

type Config struct {
	BatchSize     int
	FetchTimeout  time.Duration
	BatchInterval time.Duration
	MaxRetry      int
	RetryBackoff  time.Duration
}

func NewRunner(db *sql.DB, adp adapter.StatsAdapter, cfg Config) *Runner {
	return &Runner{db: db, adapter: adp, cfg: cfg}
}

func (r *Runner) Run(ctx context.Context) error {
	logger := logging.NewLogger("IntervalCycle", logging.WithTaskID("scheduler_cycle"))
	cycleStart := time.Now().Truncate(time.Minute)
	logger.Info("cycle start: snapshot_at=%s", cycleStart.Format(time.RFC3339))

	posts, err := r.fetchPosts(ctx)
	if err != nil {
		return fmt.Errorf("fetch posts: %w", err)
	}
	logger.Info("found %d posts to pull", len(posts))

	var pulled, failed int
	for i := 0; i < len(posts); i += r.cfg.BatchSize {
		end := i + r.cfg.BatchSize
		if end > len(posts) {
			end = len(posts)
		}
		batch := posts[i:end]

		for _, p := range batch {
			if err := r.pullOne(ctx, p, cycleStart); err != nil {
				logger.Warn(logging.WarnServiceDegraded, "pull failed: post_id=%s platform=%s err=%v", p.PostID, p.Platform, err)
				metrics.StatsPullFailTotal.WithLabelValues(p.Platform, "fetch_error").Inc()
				failed++
				continue
			}
			pulled++
		}

		if end < len(posts) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(r.cfg.BatchInterval):
			}
		}
	}

	metrics.SchedulerCycleTotal.Inc()
	metrics.SchedulerLastSuccessTimestamp.Set(float64(time.Now().Unix()))
	logger.Info("cycle done: pulled=%d failed=%d duration=%s",
		pulled, failed, time.Since(cycleStart))
	logger.Close()
	return nil
}

type postRecord struct {
	PostID   string
	Platform string
}

func (r *Runner) fetchPosts(ctx context.Context) ([]postRecord, error) {
	cutoff := time.Now().AddDate(0, 0, -30)
	rows, err := r.db.QueryContext(ctx,
		`SELECT post_id, platform FROM publish_record
		 WHERE status = 'ok'
		   AND post_id IS NOT NULL AND post_id != ''
		   AND published_at > ?
		 ORDER BY id ASC`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []postRecord
	for rows.Next() {
		var p postRecord
		if err := rows.Scan(&p.PostID, &p.Platform); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func (r *Runner) pullOne(ctx context.Context, p postRecord, snapshotAt time.Time) error {
	dup, err := r.isDuplicated(ctx, p.PostID, p.Platform, snapshotAt)
	if err != nil {
		return fmt.Errorf("idempotency check: %w", err)
	}
	if dup {
		log.Printf("[scheduler] skip duplicated post_id=%s platform=%s", p.PostID, p.Platform)
		return nil
	}

	stats, err := r.fetchWithRetry(ctx, p.PostID, p.Platform)
	if err != nil {
		return fmt.Errorf("fetch stats: %w", err)
	}

	return r.insertStats(ctx, stats, snapshotAt)
}

func (r *Runner) fetchWithRetry(ctx context.Context, postID, platform string) (*adapter.PlatformStats, error) {
	var lastErr error
	for attempt := 0; attempt <= r.cfg.MaxRetry; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(r.cfg.RetryBackoff):
			}
		}

		fetchCtx, cancel := context.WithTimeout(ctx, r.cfg.FetchTimeout)
		stats, err := r.adapter.Fetch(fetchCtx, postID, platform)
		cancel()

		if err == nil {
			return stats, nil
		}
		lastErr = err
		log.Printf("[scheduler] fetch attempt %d/%d failed post_id=%s: %v",
			attempt+1, r.cfg.MaxRetry+1, postID, err)
	}
	return nil, lastErr
}

func (r *Runner) isDuplicated(ctx context.Context, postID, platform string, snapshotAt time.Time) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM platform_stats
		 WHERE post_id = ? AND platform = ? AND snapshot_at = ?`,
		postID, platform, snapshotAt,
	).Scan(&count)
	return count > 0, err
}

func (r *Runner) insertStats(ctx context.Context, stats *adapter.PlatformStats, snapshotAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO platform_stats (post_id, platform, views, likes, comments, shares, snapshot_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		stats.PostID, stats.Platform, stats.Views, stats.Likes, stats.Comments, stats.Shares, snapshotAt,
	)
	if err != nil {
		return fmt.Errorf("insert stats: %w", err)
	}
	metrics.StatsPullTotal.WithLabelValues(stats.Platform).Inc()
	return nil
}
