#!/usr/bin/env bash
# ============================================================
#  Docker 容器入口脚本 — 生产环境
#  职责: 启动 MySQL → 初始化数据库 → 启动所有服务 → 保持运行
# ============================================================
set -euo pipefail

APP_DIR="/app"

# ---------- 颜色 ----------
GREEN='\033[0;32m'
RED='\033[0;31m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${CYAN}[INFO]${NC}  $1"; }
ok()   { echo -e "  ${GREEN}OK${NC}    $1"; }
fail() { echo -e "  ${RED}FAIL${NC}  $1"; }
warn() { echo -e "  ${YELLOW}WARN${NC}  $1"; }

# ============================================================
# 1. 持久化目录
# ============================================================
setup_dirs() {
    log "准备持久化目录..."
    mkdir -p /tmp/sm_demo /tmp/logs

    # opencode 配置文件（如不存在则创建默认值）
    if [ ! -f /tmp/sm_demo/opencode_config.json ]; then
        cat > /tmp/sm_demo/opencode_config.json << 'CONFEOF'
{"permission":{"edit":"allow","bash":"deny","write":"allow","read":"allow","external_directory":"deny","doom_loop":"allow"},"skills":{"paths":["/tmp/sm_demo/skills"]}}
CONFEOF
    fi
    ok "持久化目录就绪"
}

# ============================================================
# 2. MySQL 启动与初始化
# ============================================================
start_mysql() {
    log "启动 MySQL..."

    local MYSQL_SOCKET="/var/run/mysqld/mysqld.sock"
    local MYSQL_PASS="${MYSQL_ROOT_PASSWORD:-claw123}"

    # 确保 MySQL socket 目录存在并修正权限
    mkdir -p /var/run/mysqld
    chown -R mysql:mysql /var/run/mysqld

    # 修正 volume mount 数据目录的权限
    chown -R mysql:mysql /var/lib/mysql
    chmod 1777 /tmp

    # 确保 MySQL 数据目录存在
    if [ ! -d /var/lib/mysql/mysql ]; then
        warn "MySQL 数据目录未初始化，正在初始化..."
        if mysqld --initialize-insecure --user=mysql --datadir=/var/lib/mysql 2>&1; then
            ok "MySQL 数据目录初始化完成"
        else
            fail "MySQL 初始化失败"
            return 1
        fi
    fi

    # 启动 mysqld（显式指定 socket）
    mysqld --user=mysql --datadir=/var/lib/mysql --socket="$MYSQL_SOCKET" &
    local mysql_pid=$!

    # 等待 MySQL 就绪
    local retries=30
    while [ $retries -gt 0 ]; do
        if mysqladmin ping -u root --socket="$MYSQL_SOCKET" --silent 2>/dev/null; then
            ok "MySQL 已启动 (pid=$mysql_pid)"
            break
        fi
        retries=$((retries - 1))
        sleep 1
    done

    if [ $retries -eq 0 ]; then
        fail "MySQL 启动超时"
        return 1
    fi

    local MYSQL_BASE="mysql -u root --socket=$MYSQL_SOCKET"

    # 设置 root 密码
    if $MYSQL_BASE -e "ALTER USER 'root'@'localhost' IDENTIFIED BY '${MYSQL_PASS}'; FLUSH PRIVILEGES;" 2>/dev/null; then
        ok "root 密码已设置"
    else
        warn "root 密码设置失败（可能已设置过）"
    fi

    MYSQL_BASE="mysql -u root -p${MYSQL_PASS} --socket=$MYSQL_SOCKET"

    # 创建 xlongxia 数据库用户
    log "创建数据库用户..."
    $MYSQL_BASE -e "CREATE USER IF NOT EXISTS 'xlongxia'@'%' IDENTIFIED BY 'Xlongxia_123';" 2>/dev/null || true
    $MYSQL_BASE -e "GRANT ALL PRIVILEGES ON xlongxia.* TO 'xlongxia'@'%';" 2>/dev/null || true
    $MYSQL_BASE -e "FLUSH PRIVILEGES;" 2>/dev/null || true
    ok "数据库用户就绪"

    # 初始化数据库 Schema
    init_databases
}

init_databases() {
    local MYSQL_SOCKET="/var/run/mysqld/mysqld.sock"
    local MYSQL_PASS="${MYSQL_ROOT_PASSWORD:-claw123}"
    local mysql_cmd="mysql -u root -p${MYSQL_PASS} --socket=$MYSQL_SOCKET"

    # xlongxia 数据库
    if [ -f "$APP_DIR/schema_xlongxia.sql" ]; then
        if $mysql_cmd < "$APP_DIR/schema_xlongxia.sql" 2>/tmp/db_err.log; then
            ok "xlongxia 数据库已初始化"
        else
            if grep -qi "already exists" /tmp/db_err.log 2>/dev/null; then
                log "  xlongxia 已存在，跳过"
            else
                warn "xlongxia 初始化失败（可能已存在）"
            fi
        fi
    fi

    # claw_studios 数据库
    if [ -f "$APP_DIR/schema_claw_studios.sql" ]; then
        if $mysql_cmd < "$APP_DIR/schema_claw_studios.sql" 2>/tmp/db_err.log; then
            ok "claw_studios 数据库已初始化"
        else
            if grep -qi "already exists" /tmp/db_err.log 2>/dev/null; then
                log "  claw_studios 已存在，跳过"
            else
                warn "claw_studios 初始化失败（可能已存在）"
            fi
        fi
    fi

    # 增量迁移
    if [ -d "$APP_DIR/migrations" ]; then
        for f in $(ls "$APP_DIR/migrations"/*.sql 2>/dev/null | sort); do
            local fname=$(basename "$f")
            if $mysql_cmd < "$f" 2>/tmp/migrate_err.log; then
                ok "migration: $fname"
            else
                if grep -qi "duplicate column\|already exists\|duplicate key\|Duplicate entry" /tmp/migrate_err.log 2>/dev/null; then
                    log "  migration $fname 已执行过，跳过"
                else
                    warn "migration $fname 失败（可能已执行过）"
                fi
            fi
        done
    fi
}

# ============================================================
# 3. 启动后端服务（按 start_all.sh 顺序）
# ============================================================

wait_health() {
    local url="$1" label="$2" max_retries="${3:-15}"
    local i=0
    while [ $i -lt $max_retries ]; do
        if curl -s --max-time 3 "$url" > /dev/null 2>&1; then
            ok "$label"
            return 0
        fi
        sleep 1
        i=$((i + 1))
    done
    fail "$label (healthcheck 超时)"
    return 1
}

start_skills_register() {
    log "启动 Skills Register (:18090)..."
    cd "$APP_DIR/L1_skills_register"
    ./skill_registry --port 18090 --internal-auth="" \
        --skill-dir "$APP_DIR/L1_skills_register/fixtures" \
        --cover-bin "$APP_DIR/L1_novel_cover_png/novelcover_pure" \
        --fonts-dir "$APP_DIR/L1_novel_cover_png/fonts" \
        > /tmp/sr.log 2>&1 &
    wait_health "http://127.0.0.1:18090/api/skill/status" "Skills Register :18090"
}

start_ai_provider() {
    log "启动 AI Provider (:18180)..."
    cd "$APP_DIR/L1_AI_Provider"
    ./ai_provider --port 18180 --config-path /tmp/sm_demo/opencode_config.json \
        > /tmp/ap.log 2>&1 &
    wait_health "http://127.0.0.1:18180/healthz" "AI Provider :18180"
}

start_session_manager() {
    log "启动 Session Manager (:18080)..."
    cd "$APP_DIR/L2_conversion_manager"
    ./session_manager \
        --port 18080 \
        --data-dir /tmp/sm_demo \
        --max-concurrent 2 \
        --default-timeout-sec 600 \
        --stale-timeout-min 60 \
        --skill-registry http://localhost:18090 \
        > /tmp/sm.log 2>&1 &
    wait_health "http://127.0.0.1:18080/api/status" "Session Manager :18080"
}

start_a1_vault() {
    log "启动 A1 Account Vault (:8084)..."
    cd "$APP_DIR/L0_AI_Account_Secret_Vault"
    ./a1_server > /tmp/a1.log 2>&1 &
    wait_health "http://127.0.0.1:8084/healthz" "A1 Account Vault :8084"
}

start_workflow_engine() {
    log "启动 Workflow Engine (:9100)..."
    cd "$APP_DIR/L2_AI_Workflow_Engine"
    ./workflow_engine > /tmp/wf.log 2>&1 &
    wait_health "http://127.0.0.1:9100/health" "Workflow Engine :9100"
}

start_dashboard() {
    log "启动 Dashboard (:8083)..."
    cd "$APP_DIR/L1_AI_Dashboard"
    ./c2_dashboard > /tmp/c2.log 2>&1 &
    wait_health "http://127.0.0.1:8083/health" "Dashboard :8083"
}

start_scheduler() {
    log "启动 Interval Scheduler (:9104)..."
    cd "$APP_DIR/L2_AI_Interval"

    local cfg_path="/tmp/scheduler_cfg.yaml"
    cat > "$cfg_path" << YAMLEOF
scheduler:
  cron_expr: "0,30 * * * *"
  batch_size: 10
  fetch_timeout: 30s
  batch_interval: 200ms
  lookback_days: 30
  max_retry: 1
  retry_backoff: 1s
  listen_port: 9104
database:
  dsn: "${A1_DB_DSN}"
YAMLEOF

    env SCHEDULER_CONFIG_PATH="$cfg_path" SCHEDULER_USE_MOCK=true \
        ./scheduler > /tmp/scheduler.log 2>&1 &
    wait_health "http://127.0.0.1:9104/healthz" "Interval Scheduler :9104"
}

start_bff() {
    log "启动 BFF Gateway (:8088)..."
    cd "$APP_DIR/L3_AI_BFF"
    export SESSION_MGR_URL="${SESSION_MGR_URL:-http://localhost:18080}"
    export WORKFLOW_URL="${WORKFLOW_URL:-http://localhost:9100}"
    export C2_DASHBOARD_URL="${C2_DASHBOARD_URL:-http://localhost:8083}"
    export A1_ACCOUNT_URL="${A1_ACCOUNT_URL:-http://localhost:8084}"
    export SKILL_REGISTRY_URL="${SKILL_REGISTRY_URL:-http://localhost:18090}"
    export AI_MODEL_URL="${AI_MODEL_URL:-http://localhost:18180}"
    export A1_BASE_URL="${A1_BASE_URL:-http://localhost:8084}"
    export DB_DSN="${DB_DSN:-root:claw123@tcp(127.0.0.1:3306)/claw_studios?parseTime=true&charset=utf8mb4}"
    export A4_STORAGE_DIR="${A4_STORAGE_DIR:-/tmp/sm_demo}"
    export STOPPED_TASKS_FILE="${STOPPED_TASKS_FILE:-/tmp/sm_demo/stopped_tasks.json}"
    export JWT_SECRET="${JWT_SECRET:-not-default-secret-change-me}"
    ./bff-server > /tmp/bff.log 2>&1 &
    wait_health "http://127.0.0.1:8088/healthz" "BFF Gateway :8088"
}

start_frontend() {
    log "启动 Frontend (:${FE_PORT:-3000})..."
    cd "$APP_DIR/Front_design"

    if [ ! -d ".next" ]; then
        fail ".next 目录不存在，前端无法启动"
        return 1
    fi

    npm run start > /tmp/fe.log 2>&1 &
    local fe_pid=$!

    # 等待前端就绪
    local max_wait=30
    local elapsed=0
    while [ "$elapsed" -lt "$max_wait" ]; do
        if ! kill -0 "$fe_pid" 2>/dev/null; then
            fail "Frontend 进程已退出（详见 /tmp/fe.log）"
            return 1
        fi
        if grep -qE "Ready|ready|started server|Listening on" /tmp/fe.log 2>/dev/null; then
            if curl -s -o /dev/null --max-time 5 "http://127.0.0.1:${FE_PORT:-3000}/" 2>/dev/null; then
                ok "Frontend :${FE_PORT:-3000}"
                return 0
            fi
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done

    fail "Frontend 启动超时 (${max_wait}s，详见 /tmp/fe.log)"
    return 1
}

# ============================================================
# 主流程
# ============================================================
main() {
    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}  全模块后端 + 前端  Docker 部署${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""

    setup_dirs
    start_mysql

    echo ""
    echo -e "${CYAN}--- 大模块1: 文档编写 ---${NC}"
    start_skills_register    || true
    start_ai_provider        || true
    start_session_manager    || true

    echo ""
    echo -e "${CYAN}--- 大模块2: 发布沉淀 ---${NC}"
    start_a1_vault           || true
    start_workflow_engine    || true

    echo ""
    echo -e "${CYAN}--- 大模块3: 调度看板 ---${NC}"
    start_dashboard          || true
    start_scheduler          || true

    echo ""
    echo -e "${CYAN}--- 大模块5: BFF 网关 ---${NC}"
    start_bff                || true

    echo ""
    echo -e "${CYAN}--- 大模块6: 前端 ---${NC}"
    start_frontend           || true

    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${GREEN}所有服务启动完成${NC}"
    echo ""
    echo "服务端口对照:"
    echo "  :18080  Session Manager      (会话管家)"
    echo "  :18090  Skills Register      (写作风格仓库)"
    echo "  :18180  AI Provider          (API Key钱包)"
    echo "  :8083   Dashboard            (看板查询)"
    echo "  :8084   A1 Account Vault     (账号凭证加密)"
    echo "  :8088   BFF Gateway          (前端统一入口)"
    echo "  :9100   Workflow Engine      (发布工作流)"
    echo "  :9104   Interval Scheduler   (定时调度器)"
    echo "  :3000   Frontend             (Next.js 前端)"
    echo ""
    echo "前置统一入口: http://localhost:8088"
    echo "前端UI入口:   http://localhost:3000"
    echo ""

    # 保持容器运行 — 监控 BFF 进程
    wait
}

main
