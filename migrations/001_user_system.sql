-- 001_user_system.sql
-- 用户系统基础 migration：角色、Token 失效、管理员审计

ALTER TABLE a1_users
  ADD COLUMN role VARCHAR(16) NOT NULL DEFAULT 'user'
  AFTER password;

ALTER TABLE a1_users
  ADD COLUMN password_changed_at TIMESTAMP NULL
  AFTER role;

-- 管理员操作审计日志表
CREATE TABLE IF NOT EXISTS admin_audit_log (
  id         BIGINT AUTO_INCREMENT PRIMARY KEY,
  operator   VARCHAR(64)  NOT NULL,
  action     VARCHAR(32)  NOT NULL,
  target_uid VARCHAR(64)  NOT NULL,
  detail     VARCHAR(256),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_operator (operator),
  INDEX idx_target (target_uid)
);
