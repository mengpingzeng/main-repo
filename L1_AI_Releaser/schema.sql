-- publish_record: 平台发布记录表
-- C1 发布模块写入，C2 看板模块读取
CREATE TABLE IF NOT EXISTS publish_record (
    id           BIGINT       NOT NULL AUTO_INCREMENT COMMENT '自增主键',
    task_id      VARCHAR(64)  NOT NULL COMMENT '任务 ID，格式 task_xxxxxxxx',
    account_id   VARCHAR(64)  NOT NULL COMMENT '账号 ID，来自 A1 account_credentials.account_id',
    uid          VARCHAR(64)  NOT NULL COMMENT '用户 ID',
    platform     VARCHAR(32)  NOT NULL COMMENT '平台标识: xhs | wechat | fanqie | douyin | zhulang',
    content_hash CHAR(64)     NOT NULL DEFAULT '' COMMENT '发布内容的 SHA-256',
    status       VARCHAR(16)  NOT NULL COMMENT '发布状态: ok | fail',
    post_id      VARCHAR(128) NOT NULL DEFAULT '' COMMENT '平台返回的帖子 ID',
    error_code   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '失败时的错误码',
    error_msg    TEXT         NOT NULL COMMENT '失败时的错误描述（不含凭证信息）',
    called_at    TIMESTAMP    NOT NULL COMMENT '发布调用的时间',
    published_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '发布时间',
    updated_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '记录更新时间',

    UNIQUE KEY uk_task_account_platform (task_id, account_id, platform),
    INDEX idx_task_id (task_id),
    INDEX idx_account_id (account_id),
    INDEX idx_uid (uid),
    INDEX idx_platform (platform),
    INDEX idx_post_id (post_id),
    INDEX idx_status (status),
    INDEX idx_called_at (called_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='平台发布记录表（C1 写入，C2 看板读取）';

-- 新增字段：Skill 来源 ID 和会话 ID（可选，上游不传则为 NULL）
ALTER TABLE publish_record
    ADD COLUMN skill_id    VARCHAR(64) DEFAULT NULL COMMENT 'Skill 标识，记录本次发布用了哪个 Skill',
    ADD COLUMN session_id  VARCHAR(64) DEFAULT NULL COMMENT '会话标识，记录本次发布属于哪一轮对话',
    CHANGE COLUMN created_at published_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '发布时间';
