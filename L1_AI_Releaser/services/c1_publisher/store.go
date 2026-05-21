// Package c1_publisher 提供发布记录的持久化存储。
package c1_publisher

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

// Store 发布记录存储接口。内存实现用于测试，MySQL 实现用于生产。
type Store interface {
	Upsert(ctx context.Context, record *PublishRecord) error
	FindByTaskID(ctx context.Context, taskID string) ([]*PublishRecord, error)
	FindByPostID(ctx context.Context, postID string) (*PublishRecord, error)
}

// PublishStore 发布记录存储接口。
type PublishStore struct {
	mu   sync.RWMutex
	data map[string]*PublishRecord // key: taskID|accountID|platform
}

// NewPublishStore 创建内存存储实例（P0 用内存，P1 切换 MySQL）。
func NewPublishStore() *PublishStore {
	return &PublishStore{
		data: make(map[string]*PublishRecord),
	}
}

// Upsert 插入或覆盖更新发布记录。
// 同一 task_id + account_id + platform → 覆盖更新。
func (s *PublishStore) Upsert(_ context.Context, record *PublishRecord) error {
	key := record.TaskID + "|" + record.AccountID + "|" + record.Platform
	now := time.Now().UTC()
	record.UpdatedAt = now

	s.mu.Lock()
	existing, exists := s.data[key]
	if exists {
		record.PublishedAt = existing.PublishedAt
		record.ID = existing.ID
	}
	if record.PublishedAt.IsZero() {
		record.PublishedAt = now
	}
	s.data[key] = record
	s.mu.Unlock()
	return nil
}

// FindByTaskID 按 task_id 查询所有发布记录。
func (s *PublishStore) FindByTaskID(_ context.Context, taskID string) ([]*PublishRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var records []*PublishRecord
	for _, r := range s.data {
		if r.TaskID == taskID {
			records = append(records, r)
		}
	}
	return records, nil
}

// FindByPostID 按 post_id 查询（供定时调度器使用）。
func (s *PublishStore) FindByPostID(_ context.Context, postID string) (*PublishRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.data {
		if r.PostID == postID {
			return r, nil
		}
	}
	return nil, nil
}

// Count 返回存储中的记录总数（测试用）。
func (s *PublishStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// MySQLPublishStore 基于 MySQL 的发布记录存储。
// 生产环境使用，支持持久化和崩溃恢复。
type MySQLPublishStore struct {
	db *sql.DB
}

// NewMySQLPublishStore 创建 MySQL 发布记录存储。
func NewMySQLPublishStore(db *sql.DB) *MySQLPublishStore {
	return &MySQLPublishStore{db: db}
}

// Upsert 插入或覆盖更新发布记录。
// 使用 INSERT ... ON DUPLICATE KEY UPDATE 实现幂等。
func (m *MySQLPublishStore) Upsert(ctx context.Context, record *PublishRecord) error {
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO publish_record
			(task_id, account_id, uid, platform, content_hash, status,
			 post_id, error_code, error_msg, novel_name, skill_id, session_id)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		 ON DUPLICATE KEY UPDATE
			status=VALUES(status), post_id=VALUES(post_id),
			error_code=VALUES(error_code), error_msg=VALUES(error_msg),
			novel_name=VALUES(novel_name), updated_at=NOW()`,
		record.TaskID, record.AccountID, record.UID, record.Platform,
		record.ContentHash, record.Status, record.PostID,
		record.ErrorCode, record.ErrorMsg, record.NovelName,
		record.SkillID, record.SessionID)
	return err
}

// FindByTaskID 按 task_id 查询所有发布记录。
func (m *MySQLPublishStore) FindByTaskID(ctx context.Context, taskID string) ([]*PublishRecord, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT id, task_id, account_id, uid, platform, content_hash,
		        status, post_id, error_code, error_msg, novel_name,
		        skill_id, session_id, called_at, published_at, updated_at
		 FROM publish_record WHERE task_id=?`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*PublishRecord
	for rows.Next() {
		r := &PublishRecord{}
		if err := rows.Scan(&r.ID, &r.TaskID, &r.AccountID, &r.UID, &r.Platform,
			&r.ContentHash, &r.Status, &r.PostID, &r.ErrorCode, &r.ErrorMsg,
			&r.NovelName, &r.SkillID, &r.SessionID,
			&r.CalledAt, &r.PublishedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// FindByPostID 按 post_id 查询发布记录。
func (m *MySQLPublishStore) FindByPostID(ctx context.Context, postID string) (*PublishRecord, error) {
	row := m.db.QueryRowContext(ctx,
		`SELECT id, task_id, account_id, uid, platform, content_hash,
		        status, post_id, error_code, error_msg, novel_name,
		        skill_id, session_id, called_at, published_at, updated_at
		 FROM publish_record WHERE post_id=?`, postID)
	r := &PublishRecord{}
	err := row.Scan(&r.ID, &r.TaskID, &r.AccountID, &r.UID, &r.Platform,
		&r.ContentHash, &r.Status, &r.PostID, &r.ErrorCode, &r.ErrorMsg,
		&r.NovelName, &r.SkillID, &r.SessionID,
		&r.CalledAt, &r.PublishedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}
