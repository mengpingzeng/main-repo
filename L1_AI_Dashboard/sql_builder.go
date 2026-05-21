package c2_dashboard

import (
	"strings"
	"time"
)

const baseQuery = `
SELECT
    pr.post_id,
    pr.account_id,
    pr.platform,
    COALESCE(pr.skill_id,   '') AS skill_id,
    COALESCE(pr.session_id, '') AS session_id,
    COALESCE(pr.novel_name,  '') AS novel_name,
    COALESCE(ac.masked_display, '') AS login_name,
    COALESCE(ps.views,    0) AS views,
    COALESCE(ps.likes,    0) AS likes,
    COALESCE(ps.comments, 0) AS comments,
    COALESCE(ps.shares,   0) AS shares,
    pr.published_at
FROM publish_record pr
LEFT JOIN a1_credentials ac ON ac.account_id = pr.account_id
LEFT JOIN (
    SELECT post_id, views, likes, comments, shares
    FROM platform_stats
    WHERE (post_id, snapshot_at) IN (
        SELECT post_id, MAX(snapshot_at)
        FROM platform_stats
        GROUP BY post_id
    )
) ps ON ps.post_id = pr.post_id
WHERE pr.status = 'ok'
  AND pr.post_id IS NOT NULL
  AND pr.post_id != ''
  AND (pr.novel_name, pr.published_at) IN (
      SELECT novel_name, MAX(published_at)
      FROM publish_record
      WHERE status = 'ok' AND post_id IS NOT NULL AND post_id != ''
      GROUP BY novel_name
  )`

func buildQuery(req DashboardQueryRequest) (string, []interface{}, error) {
	query := baseQuery
	var args []interface{}

	query += buildInClause("pr.account_id", req.AccountIDs, &args)
	query += buildInClause("pr.platform", req.Platforms, &args)
	query += buildInClause("pr.skill_id", req.SkillIDs, &args)
	query += buildInClause("pr.session_id", req.SessionIDs, &args)

	if req.UID != "" {
		query += " AND pr.account_id IN (SELECT account_id FROM a1_credentials WHERE uid = ?)"
		args = append(args, req.UID)
	}

	if req.StartTime != "" {
		t, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			return "", nil, ErrInvalidTimeRange
		}
		query += " AND pr.published_at >= ?"
		args = append(args, t)
	}

	if req.EndTime != "" {
		t, err := time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			return "", nil, ErrInvalidTimeRange
		}
		query += " AND pr.published_at <= ?"
		args = append(args, t)
	}

	query += " ORDER BY pr.published_at DESC"

	return query, args, nil
}

func buildInClause(column string, values []string, args *[]interface{}) string {
	if len(values) == 0 {
		return ""
	}
	placeholders := make([]string, len(values))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	for _, v := range values {
		*args = append(*args, v)
	}
	return " AND " + column + " IN (" + strings.Join(placeholders, ",") + ")"
}
