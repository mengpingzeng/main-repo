-- Workflow Engine 生产建表 SQL
-- 用法: mysql -u xlongxia -p'Xlongxia_123' xlongxia < schema.sql
-- 表名: workflow_task，与 types.go WorkflowTask 结构体字段完全对齐

CREATE TABLE IF NOT EXISTS workflow_task (
    task_id          VARCHAR(64) PRIMARY KEY            COMMENT '任务唯一标识',
    uid              VARCHAR(64) NOT NULL                COMMENT '用户 ID',
    skill_id         VARCHAR(64) DEFAULT ''             COMMENT 'Skill ID',
    topic            VARCHAR(512) DEFAULT ''            COMMENT '任务主题',
    platform         VARCHAR(32) NOT NULL                COMMENT '目标平台（fanqie/xhs/wechat/zhulang）',
    status           ENUM('init','fetch_draft','publishing','published',
                          'md_writing','md_written','done','done_partial',
                          'failed_gen','failed_md') NOT NULL DEFAULT 'init'
                                                        COMMENT '任务状态',
    session_id       VARCHAR(64) NOT NULL                COMMENT 'Agent Session ID',
    draft_version    INT DEFAULT 0                      COMMENT '草稿版本号',
    md_path          VARCHAR(512) DEFAULT ''            COMMENT 'MD 档案 OSS/本地路径',
    trace_id         VARCHAR(64) DEFAULT ''             COMMENT '全链路 Trace ID',
    publish_results  JSON                               COMMENT '发布结果 JSON 数组',
    accounts         JSON                               COMMENT '发布目标账号 JSON 数组',
    current_step     VARCHAR(32) DEFAULT ''             COMMENT '当前执行步骤',
    step_retry       INT DEFAULT 0                      COMMENT '当前步骤重试次数',
    step_updated_at  TIMESTAMP NULL                     COMMENT '步骤更新时间',
    error_msg        TEXT                               COMMENT '错误信息',
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
                                                        COMMENT '更新时间',

    INDEX idx_status (status),
    INDEX idx_recover (status, step_updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Workflow Engine 发布流水线任务表';
