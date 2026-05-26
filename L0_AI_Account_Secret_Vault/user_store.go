package vault

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type UserStore struct {
	db *sql.DB
}

func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

func generateUID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "usr_" + hex.EncodeToString(b)
}

func (s *UserStore) hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func (s *UserStore) checkPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (s *UserStore) Create(ctx context.Context, username, password, role string) (*User, error) {
	existing, err := s.FindByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("check existing user: %w", err)
	}
	if existing != nil {
		return nil, ErrUserExists
	}

	if len(password) < 8 {
		return nil, ErrPasswordTooShort
	}

	if role == "" {
		role = "user"
	}

	hash, err := s.hashPassword(password)
	if err != nil {
		return nil, err
	}

	uid := generateUID()
	query := `INSERT INTO a1_users (uid, username, password, role, password_changed_at, created_at, updated_at) VALUES (?, ?, ?, ?, NOW(), NOW(), NOW())`
	_, err = s.db.ExecContext(ctx, query, uid, username, hash, role)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return &User{
		UID:      uid,
		Username: username,
		Password: hash,
		Role:     role,
	}, nil
}

func (s *UserStore) FindByUsername(ctx context.Context, username string) (*User, error) {
	query := `SELECT uid, username, password, role, password_changed_at, created_at, updated_at FROM a1_users WHERE username = ?`
	user := &User{}
	err := s.db.QueryRowContext(ctx, query, username).Scan(
		&user.UID, &user.Username, &user.Password, &user.Role, &user.PasswordChangedAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by username: %w", err)
	}
	return user, nil
}

func (s *UserStore) FindByUID(ctx context.Context, uid string) (*User, error) {
	query := `SELECT uid, username, password, role, password_changed_at, created_at, updated_at FROM a1_users WHERE uid = ?`
	user := &User{}
	err := s.db.QueryRowContext(ctx, query, uid).Scan(
		&user.UID, &user.Username, &user.Password, &user.Role, &user.PasswordChangedAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by uid: %w", err)
	}
	return user, nil
}

func (s *UserStore) Authenticate(ctx context.Context, username, password string) (*User, error) {
	user, err := s.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}
	if !s.checkPassword(user.Password, password) {
		return nil, ErrInvalidCredentials
	}
	// 异步更新登录时间，用独立 context 避免请求结束后 ctx 被取消
	go func() {
		_, _ = s.db.ExecContext(context.Background(),
			`UPDATE a1_users SET last_login_at = NOW() WHERE uid = ?`,
			user.UID,
		)
	}()
	return user, nil
}

func (s *UserStore) ListUsers(ctx context.Context, page, size int, priorityUID string) ([]AdminUserInfo, int, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 5
	}
	if size > 100 {
		size = 100
	}
	offset := (page - 1) * size

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM a1_users`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	query := `
		SELECT u.uid, u.username, u.role, u.created_at,
		       COALESCE(c.account_count, 0) AS account_count,
		       COALESCE(t.task_count, 0) AS task_count,
		       u.last_login_at
		FROM a1_users u
		LEFT JOIN (
			SELECT uid, COUNT(*) AS account_count
			FROM a1_credentials
			WHERE credential IS NOT NULL AND credential != ''
			GROUP BY uid
		) c ON c.uid COLLATE utf8mb4_unicode_ci = u.uid
		LEFT JOIN (
			SELECT uid, COUNT(*) AS task_count
			FROM workflow_task
			GROUP BY uid
		) t ON t.uid COLLATE utf8mb4_unicode_ci = u.uid
		ORDER BY (u.uid = ?) DESC, u.created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := s.db.QueryContext(ctx, query, priorityUID, size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []AdminUserInfo
	for rows.Next() {
		var u AdminUserInfo
		var createdAt time.Time
		var lastLoginAt sql.NullTime
		if err := rows.Scan(&u.UID, &u.Username, &u.Role, &createdAt, &u.AccountCount, &u.TaskCount, &lastLoginAt); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		u.CreatedAt = createdAt.Format(time.RFC3339)
		if lastLoginAt.Valid {
			u.LastLoginAt = lastLoginAt.Time.Format(time.RFC3339)
		}
		users = append(users, u)
	}
	if users == nil {
		users = []AdminUserInfo{}
	}
	return users, total, rows.Err()
}

func (s *UserStore) UpdateUser(ctx context.Context, uid, password, role string, operatorUID string) error {
	user, err := s.FindByUID(ctx, uid)
	if err != nil {
		return fmt.Errorf("find user: %w", err)
	}
	if user == nil {
		return nil
	}

	if operatorUID == uid && role != "" && role != user.Role {
		return ErrCannotChangeOwnRole
	}

	if password != "" {
		if len(password) < 8 {
			return ErrPasswordTooShort
		}
		hash, err := s.hashPassword(password)
		if err != nil {
			return err
		}
		if role != "" {
			_, err = s.db.ExecContext(ctx,
				`UPDATE a1_users SET password = ?, role = ?, password_changed_at = NOW(), updated_at = NOW() WHERE uid = ?`,
				hash, role, uid)
		} else {
			_, err = s.db.ExecContext(ctx,
				`UPDATE a1_users SET password = ?, password_changed_at = NOW(), updated_at = NOW() WHERE uid = ?`,
				hash, uid)
		}
		return err
	}

	if role != "" {
		_, err = s.db.ExecContext(ctx,
			`UPDATE a1_users SET role = ?, updated_at = NOW() WHERE uid = ?`,
			role, uid)
		return err
	}

	return nil
}

func (s *UserStore) DeleteUser(ctx context.Context, uid, operatorUID string) error {
	if uid == operatorUID {
		return ErrCannotDeleteSelf
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// 删除用户前，一并解除其所有绑定账号
	if _, err := tx.ExecContext(ctx,
		`UPDATE a1_credentials SET credential = '' WHERE uid = ? AND credential IS NOT NULL AND credential != ''`,
		uid,
	); err != nil {
		return fmt.Errorf("unbind accounts: %w", err)
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM a1_users WHERE uid = ?`, uid)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil
	}

	return tx.Commit()
}

func (s *UserStore) CountUserAccounts(ctx context.Context, uid string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM a1_credentials WHERE uid = ? AND credential IS NOT NULL AND credential != ''`,
		uid).Scan(&count)
	return count, err
}
