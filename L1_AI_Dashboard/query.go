package c2_dashboard

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"clawstudios/pkg/logging"
)

// Querier 看板查询接口（唯一对外暴露的方法）。
type Querier interface {
	Query(ctx context.Context, req DashboardQueryRequest) (*DashboardQueryResponse, error)
	Health(ctx context.Context) error
}

type dashboardQuerier struct {
	db *sql.DB
}

// New 创建看板查询器。构造函数只接受 *sql.DB，体现"纯读 MySQL、不依赖任何其他模块"。
func New(db *sql.DB) Querier {
	return &dashboardQuerier{db: db}
}

func (q *dashboardQuerier) Query(ctx context.Context, req DashboardQueryRequest) (*DashboardQueryResponse, error) {
	logger := logging.FromContext(ctx)
	startTime := time.Now()

	if logger != nil {
		logger.Info("dashboard query: uid=%s task=%s platforms=%v page=%d size=%d",
			req.UID, req.TaskID, req.Platforms, req.Page, req.Size)
	}

	if err := validateRequest(req); err != nil {
		return nil, err
	}

	summaryQuery, summaryArgs, err := buildSummaryQuery(req)
	if err != nil {
		return nil, err
	}

	var summary DashboardSummary
	if err := q.db.QueryRowContext(ctx, summaryQuery, summaryArgs...).Scan(
		&summary.TotalPosts,
		&summary.TotalViews,
		&summary.TotalLikes,
		&summary.TotalComments,
		&summary.TotalShares,
	); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInternalError, err)
	}

	sqlQuery, sqlArgs, err := buildQuery(req)
	if err != nil {
		return nil, err
	}

	rows, err := q.db.QueryContext(ctx, sqlQuery, sqlArgs...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDBUnavailable, err)
	}
	defer rows.Close()

	items := make([]DashboardItem, 0)
	for rows.Next() {
		var item DashboardItem
		var publishedAt time.Time
		if err := rows.Scan(
			&item.PostID, &item.AccountID, &item.Platform,
			&item.SkillID, &item.SessionID,
			&item.NovelName, &item.LoginName,
			&item.Views, &item.Likes, &item.Comments, &item.Shares,
			&publishedAt,
		); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInternalError, err)
		}
		item.PublishedAt = publishedAt.Format(time.RFC3339)
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInternalError, err)
	}

	if items == nil {
		items = []DashboardItem{}
	}

	if logger != nil {
		logger.Info("dashboard query done: items=%d total=%d duration=%s",
			len(items), summary.TotalPosts, time.Since(startTime))
	}

	return &DashboardQueryResponse{
		Items:   items,
		Summary: summary,
		Total:   summary.TotalPosts,
	}, nil
}

func (q *dashboardQuerier) Health(_ context.Context) error {
	return q.db.Ping()
}
