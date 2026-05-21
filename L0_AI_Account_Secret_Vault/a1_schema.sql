-- a1_credentials: 账号凭证主表
CREATE TABLE IF NOT EXISTS a1_credentials (
    account_id  VARCHAR(64)  NOT NULL PRIMARY KEY COMMENT '账号唯一标识，格式 acc_xxxxxxxx',
    uid         VARCHAR(64)  NOT NULL COMMENT '用户唯一标识',
    platform    VARCHAR(32)  NOT NULL COMMENT '平台标识: fanqie | wechat | douyin | bilibili | zhulang',
    credential  TEXT         NOT NULL COMMENT '加密后的凭证密文（Base64 编码）',
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',

    UNIQUE KEY uk_account_id (account_id),
    INDEX idx_uid (uid),
    INDEX idx_platform (platform)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='账号凭证加密存储表';

-- a1_users: 用户表
CREATE TABLE IF NOT EXISTS a1_users (
    uid         VARCHAR(64)  NOT NULL PRIMARY KEY COMMENT '用户唯一标识',
    username    VARCHAR(128) NOT NULL COMMENT '用户名',
    password    VARCHAR(256) NOT NULL COMMENT 'bcrypt 密码哈希',
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '注册时间',
    updated_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
    UNIQUE KEY uk_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='用户认证表';

-- credential_audit_log: 凭证操作审计日志
CREATE TABLE IF NOT EXISTS credential_audit_log (
    id             BIGINT       NOT NULL AUTO_INCREMENT PRIMARY KEY,
    account_id     VARCHAR(64)  NOT NULL COMMENT '关联的账号 ID',
    action         VARCHAR(32)  NOT NULL COMMENT '操作类型: bind | get_credentials | bind_denied | get_credentials_denied',
    caller         VARCHAR(128) NOT NULL COMMENT '调用方标识: bff | c1_publisher | admin_uid:xxx',
    result         VARCHAR(16)  NOT NULL COMMENT '操作结果: success | forbidden | error',
    error_code     VARCHAR(64)  DEFAULT NULL COMMENT '失败时的错误码，如 KMS_UNAVAILABLE',
    created_at     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '操作时间',

    INDEX idx_account_id (account_id),
    INDEX idx_created_at (created_at),
    INDEX idx_action (action)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='凭证操作审计日志（不记录任何凭证内容）';
