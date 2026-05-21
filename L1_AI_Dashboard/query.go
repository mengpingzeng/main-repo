package c2_dashboard

import (
	"context"
	"database/sql"
	"fmt"
	"time"
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
	if err := validateRequest(req); err != nil {
		return nil, err
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

	summary := computeSummary(items)

	return &DashboardQueryResponse{
		Items:   items,
		Summary: summary,
	}, nil
}

func (q *dashboardQuerier) Health(_ context.Context) error {
	return q.db.Ping()
}
