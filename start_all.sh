#!/usr/bin/env bash
# ============================================================
#  全模块后端 + 前端服务启动脚本 (main-repo 版本)
#  用法: bash start_all.sh
#  要求: 以 admin 用户身份在 main-repo 根目录下执行
# ============================================================
set -euo pipefail

# ========== 运行时用户 ==========
RUN_USER="admin"

# ========== 路径常量（相对于脚本所在目录） ==========
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SR_DIR="$SCRIPT_DIR/L1_skills_register"
AP_DIR="$SCRIPT_DIR/L1_AI_Provider"
SM_DIR="$SCRIPT_DIR/L2_conversion_manager"
A1_DIR="$SCRIPT_DIR/L0_AI_Account_Secret_Vault"
DB_DIR="$SCRIPT_DIR/L1_AI_Dashboard"
WF_DIR="$SCRIPT_DIR/L2_AI_Workflow_Engine"
SCHEDULER_DIR="$SCRIPT_DIR/L2_AI_Interval"
BFF_DIR="$SCRIPT_DIR/L3_AI_BFF"
FE_DIR="$SCRIPT_DIR/Front_design"
FE_PORT="${FE_PORT:-3000}"
COVER_DIR="$SCRIPT_DIR/L1_novel_cover_png"
MIGRATIONS_DIR="$SCRIPT_DIR/migrations"

DATA_DIR="/tmp/sm_demo"
LOG_DIR="/tmp/logs"

as_run_user() {
    env "$@"
}

# ========== 运行用户校验 ==========
ensure_run_user() {
    local current
    current="$(whoami)"
    if [ "$current" != "$RUN_USER" ]; then
        fail "请以 $RUN_USER 用户运行本脚本，当前用户: $current"
        echo "       用法: su - $RUN_USER -c 'cd $SCRIPT_DIR && bash start_all.sh'"
        exit 1
    fi
}

# ========== 前端目录权限（避免 root/sudo 构建遗留导致 EACCES） ==========
ensure_frontend_permissions() {
    local me="$RUN_USER"
    local dir fixed=0

    # 修复整个前端目录属主（含 package-lock.json 等根目录文件）
    if find "$FE_DIR" -maxdepth 1 ! -user "$me" -print -quit 2>/dev/null | grep -q .; then
        warn "  $FE_DIR 根目录存在非 $me 属主文件（含 package-lock.json 等），正在修复..."
        sudo chown -R "$me:$me" "$FE_DIR" 2>/dev/null && ok "  $FE_DIR 属主已全部修正为 $me" || warn "  chown $FE_DIR 失败，请手动检查"
        fixed=1
    fi

    cd "$FE_DIR"
    for dir in .next node_modules; do
        [ -d "$dir" ] || continue
        if find "$dir" -mindepth 1 ! -user "$me" -print -quit 2>/dev/null | grep -q .; then
            warn "  $dir 存在非 $me 属主文件（可能曾用 root/sudo 构建），正在修复..."
            if sudo chown -R "$me:$me" "$dir" 2>/dev/null; then
                ok "  $dir 属主已修正为 $me"
            else
                warn "  chown 失败，删除 $dir 以便重建..."
                sudo rm -rf "$dir" 2>/dev/null || rm -rf "$dir"
            fi
            fixed=1
        fi
    done
    [ "$fixed" -eq 0 ] || true
}

# ========== 环境变量（可覆盖） ==========
export A1_DB_DSN="${A1_DB_DSN:-xlongxia:Xlongxia_123@tcp(127.0.0.1:3306)/xlongxia?parseTime=true}"
export C2_DB_DSN="${C2_DB_DSN:-$A1_DB_DSN}"
export WF_DB_DSN="${WF_DB_DSN:-$A1_DB_DSN}"
export A1_ENCRYPTION_KEY="${A1_ENCRYPTION_KEY:-eLvMeGfepGpOUw280t7dvJTf+dkVAWn5B5dLOA4rMjk=}"
export A1_MOCK_ENCRYPTION_KEY="${A1_MOCK_ENCRYPTION_KEY:-$A1_ENCRYPTION_KEY}"
export A1_DB_USER="${A1_DB_USER:-xlongxia}"
export A1_DB_PASSWORD="${A1_DB_PASSWORD:-Xlongxia_123}"
export A1_DB_HOST="${A1_DB_HOST:-127.0.0.1}"
export A1_DB_NAME="${A1_DB_NAME:-xlongxia}"
export A1_JWT_SECRET="${A1_JWT_SECRET:-not-default-secret-change-me}"
export JWT_SECRET="${JWT_SECRET:-not-default-secret-change-me}"
# DEEPSEEK_API_KEY — 标准名；TEAM_DEEPSEEK_API_KEY — 旧名（向后兼容）
export DEEPSEEK_API_KEY="${DEEPSEEK_API_KEY:-${TEAM_DEEPSEEK_API_KEY:-}}"

export PORT="${PORT:-8088}"
export SESSION_MGR_URL="${SESSION_MGR_URL:-http://localhost:18080}"
export WORKFLOW_URL="${WORKFLOW_URL:-http://localhost:9100}"
export C2_DASHBOARD_URL="${C2_DASHBOARD_URL:-http://localhost:8083}"
export A1_ACCOUNT_URL="${A1_ACCOUNT_URL:-http://localhost:8084}"
export SKILL_REGISTRY_URL="${SKILL_REGISTRY_URL:-http://localhost:18090}"
export AI_MODEL_URL="${AI_MODEL_URL:-http://localhost:18180}"
export A1_BASE_URL="${A1_BASE_URL:-http://localhost:8084}"
export A4_STORAGE_DIR="${A4_STORAGE_DIR:-/tmp/sm_demo}"
export DB_DSN="${DB_DSN:-root:claw123@tcp(127.0.0.1:3306)/claw_studios?parseTime=true&charset=utf8mb4}"
export STOPPED_TASKS_FILE="${STOPPED_TASKS_FILE:-/tmp/sm_demo/stopped_tasks.json}"

# ========== 颜色 ==========
GREEN='\033[0;32m'
RED='\033[0;31m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${CYAN}[INFO]${NC}  $1"; }
ok()   { echo -e "  ${GREEN}OK${NC}    $1"; }
fail() { echo -e "  ${RED}FAIL${NC}  $1"; }
warn() { echo -e "  ${YELLOW}WARN${NC}  $1"; }

# ========== 包管理器检测 ==========
detect_pkg_mgr() {
    if command -v apt-get &>/dev/null; then
        echo "apt"
    elif command -v yum &>/dev/null; then
        echo "yum"
    elif command -v dnf &>/dev/null; then
        echo "dnf"
    else
        echo "unknown"
    fi
}

# ========== 自动安装 opencode ==========
ensure_opencode() {
    if command -v opencode &>/dev/null; then
        ok "opencode $(opencode --version 2>/dev/null || echo '?') (已安装)"
        return 0
    fi

    warn "opencode 未安装，正在自动安装..."

    # 确保 npm 可用
    if ! command -v npm &>/dev/null; then
        local pkg_mgr=$(detect_pkg_mgr)
        log "  npm 未安装，正在通过 $pkg_mgr 安装 Node.js..."
        case "$pkg_mgr" in
            apt)
                sudo apt-get update -qq && sudo apt-get install -y -qq nodejs npm 2>/tmp/npm_install.log || {
                    fail "Node.js 安装失败"
                    cat /tmp/npm_install.log
                    return 1
                }
                ;;
            yum|dnf)
                sudo $pkg_mgr install -y nodejs npm 2>/tmp/npm_install.log || {
                    # CentOS 7 可能需要 EPEL
                    sudo yum install -y epel-release 2>/dev/null || true
                    sudo $pkg_mgr install -y nodejs npm 2>/tmp/npm_install.log || {
                        fail "Node.js 安装失败"
                        cat /tmp/npm_install.log
                        return 1
                    }
                }
                ;;
            *)
                fail "无法自动安装 Node.js (未知包管理器)"
                echo "       请手动安装: https://nodejs.org/"
                return 1
                ;;
        esac
        ok "Node.js $(node -v 2>/dev/null), npm $(npm -v 2>/dev/null)"
    fi

    # 安装 opencode
    log "  安装 opencode..."
    if sudo npm install -g @anthropic/opencode 2>/tmp/opencode_install.log; then
        ok "opencode 安装成功 ($(opencode --version 2>/dev/null || echo 'ok'))"
        return 0
    else
        fail "opencode 安装失败"
        cat /tmp/opencode_install.log
        echo "       请手动安装: npm install -g @anthropic/opencode"
        return 1
    fi
}

# ========== 自动安装 MySQL ==========
ensure_mysql() {
    if command -v mysqld &>/dev/null || command -v mariadbd &>/dev/null; then
        local mysql_ver=$(mysqld --version 2>/dev/null || mariadbd --version 2>/dev/null || echo "installed")
        ok "MySQL/MariaDB 已安装 ($mysql_ver)"
    else
        warn "MySQL 未安装，正在自动安装..."
        local pkg_mgr=$(detect_pkg_mgr)
        case "$pkg_mgr" in
            apt)
                sudo apt-get update -qq && sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq mysql-server mysql-client 2>/tmp/mysql_install.log || {
                    fail "MySQL 安装失败"
                    cat /tmp/mysql_install.log
                    return 1
                }
                ;;
            yum|dnf)
                sudo $pkg_mgr install -y mysql-server mysql 2>/tmp/mysql_install.log || {
                    # CentOS 7 可能需要 mariadb
                    sudo $pkg_mgr install -y mariadb-server mariadb 2>/tmp/mysql_install.log || {
                        fail "MySQL 安装失败"
                        cat /tmp/mysql_install.log
                        return 1
                    }
                }
                ;;
            *)
                fail "无法自动安装 MySQL (未知包管理器)"
                echo "       请手动安装 MySQL 8.0+"
                return 1
                ;;
        esac
        ok "MySQL 安装完成"
    fi

    # 确保 MySQL 服务已启动
    if command -v systemctl &>/dev/null; then
        if ! sudo systemctl is-active --quiet mysqld 2>/dev/null && ! sudo systemctl is-active --quiet mysql 2>/dev/null && ! sudo systemctl is-active --quiet mariadb 2>/dev/null; then
            log "  启动 MySQL 服务..."
            sudo systemctl start mysqld 2>/dev/null || sudo systemctl start mysql 2>/dev/null || sudo systemctl start mariadb 2>/dev/null || {
                fail "无法启动 MySQL 服务"
                return 1
            }
            sudo systemctl enable mysqld 2>/dev/null || sudo systemctl enable mysql 2>/dev/null || sudo systemctl enable mariadb 2>/dev/null || true
        fi
        ok "MySQL 服务已启动"
    elif command -v service &>/dev/null; then
        if ! service mysqld status 2>/dev/null | grep -q "running"; then
            sudo service mysqld start 2>/dev/null || sudo service mysql start 2>/dev/null || sudo service mariadb start 2>/dev/null || true
        fi
    fi

    # 确保 mysql 客户端可用
    if ! command -v mysql &>/dev/null; then
        fail "mysql 客户端未安装"
        return 1
    fi
    ok "mysql 客户端就绪"

    return 0
}

# ========== 初始化 MySQL root 密码 ==========
setup_mysql_auth() {
    local expected_pass="${MYSQL_ROOT_PASSWORD:-claw123}"
    log "检查 MySQL root 认证..."

    # 尝试用预期密码连接
    if mysql -u root -p"$expected_pass" -e "SELECT 1" 2>/dev/null >/dev/null; then
        ok "MySQL root 密码已配置"
        return 0
    fi

    # 尝试无密码连接 (新安装的 MySQL)
    if mysql -u root -e "SELECT 1" 2>/dev/null >/dev/null; then
        warn "MySQL root 无密码，正在设置密码..."
        mysql -u root -e "ALTER USER 'root'@'localhost' IDENTIFIED BY '$expected_pass'; FLUSH PRIVILEGES;" 2>/tmp/mysql_auth.log || {
            fail "MySQL root 密码设置失败"
            cat /tmp/mysql_auth.log
            echo "       请手动设置: ALTER USER 'root'@'localhost' IDENTIFIED BY '$expected_pass';"
            return 1
        }
        ok "MySQL root 密码已设置为 '$expected_pass'"
        return 0
    fi

    # 尝试用 auth_socket (Ubuntu/Debian 默认)
    if mysql -u root --socket=/var/run/mysqld/mysqld.sock -e "SELECT 1" 2>/dev/null >/dev/null; then
        warn "MySQL root 使用 auth_socket 认证，正在改为密码认证..."
        mysql -u root --socket=/var/run/mysqld/mysqld.sock -e \
            "ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '$expected_pass'; FLUSH PRIVILEGES;" 2>/tmp/mysql_auth.log || {
            fail "MySQL 认证方式修改失败"
            cat /tmp/mysql_auth.log
            return 1
        }
        ok "MySQL root 已改为密码认证 ($expected_pass)"
        return 0
    fi

    fail "MySQL root 认证失败 — 无法自动配置"
    echo "       请手动设置 root 密码为 '$expected_pass' 或设置环境变量 MYSQL_ROOT_PASSWORD"
    return 1
}

# ========== 前端开发机 IP（该 IP 用 npm run dev，其余用 npm run start） ==========
FRONTEND_DEV_HOST="${FRONTEND_DEV_HOST:-47.107.124.45}"

detect_server_ipv4() {
    local ip=""
    for url in \
        "http://100.100.100.200/latest/meta-data/eipv4" \
        "http://169.254.169.254/latest/meta-data/public-ipv4"; do
        ip=$(curl -fsS --connect-timeout 1 --max-time 2 "$url" 2>/dev/null || true)
        if [[ "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "$ip"
            return 0
        fi
    done
    if command -v hostname &>/dev/null; then
        for ip in $(hostname -I 2>/dev/null); do
            if [[ "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]] && [ "$ip" != "127.0.0.1" ]; then
                echo "$ip"
                return 0
            fi
        done
    fi
    return 1
}

# ========== 清理旧前端（Next.js 16 监听进程为 next-server，需按端口清理） ==========
cleanup_frontend() {
    log "清理旧前端进程 (:${FE_PORT})..."
    local pid cwd

    # 1. 按端口释放（只影响 :3000，不碰后端 Go 服务端口）
    if command -v fuser &>/dev/null; then
        sudo fuser -k "${FE_PORT}/tcp" 2>/dev/null || true
    elif command -v lsof &>/dev/null; then
        local pids
        pids=$(lsof -t -i ":${FE_PORT}" -sTCP:LISTEN 2>/dev/null || true)
        if [ -n "$pids" ]; then
            sudo kill $pids 2>/dev/null || true
        fi
    fi

    # 2. 清理 Front_design 目录下残留的 npm / next 包装进程（dev、start 均适用）
    for pid in $(pgrep -f "npm run start|npm run dev|/bin/next dev|/bin/next start" 2>/dev/null || true); do
        cwd=$(readlink -f "/proc/${pid}/cwd" 2>/dev/null || true)
        if [ "$cwd" = "$FE_DIR" ]; then
            sudo kill "$pid" 2>/dev/null || true
        fi
    done

    sleep 2

    if ss -tlnp 2>/dev/null | grep -q ":${FE_PORT} "; then
        warn "  端口 :${FE_PORT} 仍被占用，再次按端口释放..."
        if command -v fuser &>/dev/null; then
            sudo fuser -k "${FE_PORT}/tcp" 2>/dev/null || true
        fi
        sleep 1
    fi

    if ss -tlnp 2>/dev/null | grep -q ":${FE_PORT} "; then
        warn "  端口 :${FE_PORT} 仍被占用，请手动检查: ss -tlnp | grep :${FE_PORT}"
    else
        ok "前端端口 :${FE_PORT} 已释放"
    fi

    sudo rm -f /tmp/fe.log 2>/dev/null || true
}

# 等待 Next.js 就绪：先看日志 Ready，再 curl 验证（dev 冷启动可能 >5s）
wait_for_frontend_ready() {
    local fe_mode="$1"
    local fe_pid="$2"
    local max_wait="${FE_STARTUP_TIMEOUT:-60}"
    local interval=2
    local elapsed=0

    if [ "$fe_mode" = "start" ]; then
        max_wait="${FE_STARTUP_TIMEOUT:-30}"
    fi

    log "  等待 Frontend 就绪（最多 ${max_wait}s，模式: ${fe_mode}）..."

    while [ "$elapsed" -lt "$max_wait" ]; do
        if ! kill -0 "$fe_pid" 2>/dev/null; then
            fail "Frontend 进程已退出（详见 /tmp/fe.log）"
            return 1
        fi

        if grep -q "EADDRINUSE" /tmp/fe.log 2>/dev/null; then
            fail "Frontend 启动失败：端口 :${FE_PORT} 被占用（详见 /tmp/fe.log）"
            return 1
        fi

        if grep -qE "Failed to compile|Error: listen EADDRINUSE|Cannot find module" /tmp/fe.log 2>/dev/null; then
            fail "Frontend 启动失败（详见 /tmp/fe.log）"
            return 1
        fi

        local log_ready=0
        if grep -qE "Ready|ready|started server|Listening on" /tmp/fe.log 2>/dev/null; then
            log_ready=1
        fi

        if [ "$log_ready" -eq 1 ] && curl -s -o /dev/null --max-time 5 "http://127.0.0.1:${FE_PORT}/" 2>/dev/null; then
            return 0
        fi

        sleep "$interval"
        elapsed=$((elapsed + interval))
    done

    fail "Frontend 启动超时 (${max_wait}s，模式: ${fe_mode}，详见 /tmp/fe.log)"
    return 1
}

# ========== 清理旧进程 ==========
cleanup_old() {
    log "清理旧进程..."
    for proc in run_bff.sh skill_registry ai_provider session_manager a1_server c2_dashboard workflow_engine scheduler bff-server; do
        sudo pkill -f "$proc" 2>/dev/null || true
    done
    cleanup_frontend
    sleep 1
}

# ========== 前置检查 ==========
check_prereq() {
    log "前置检查..."

    # --- DEEPSEEK_API_KEY ---
    # 作用: 调用 DeepSeek API 的认证密钥
    # 链路: start_all.sh → session_manager --deepseek-api-key → opencode 子进程 → DeepSeek API
    # 兼容旧变量名 TEAM_DEEPSEEK_API_KEY
    if [ -z "${DEEPSEEK_API_KEY:-}" ]; then
        fail "DEEPSEEK_API_KEY 未设置"
        echo "       ┌─────────────────────────────────────────────────────────────┐"
        echo "       │  变量名: DEEPSEEK_API_KEY                                    │"
        echo "       │  作用:   AI 写作引擎调用 DeepSeek API 的认证密钥              │"
        echo "       │  链路:   脚本 → session_manager → opencode → DeepSeek API    │"
        echo "       │  无此 Key = AI 写稿功能完全不可用                            │"
        echo "       │                                                             │"
        echo "       │  获取: https://platform.deepseek.com → API Keys             │"
        echo "       │  设置: export DEEPSEEK_API_KEY=sk-xxxxxxxx                  │"
        echo "       └─────────────────────────────────────────────────────────────┘"
    else
        ok "DEEPSEEK_API_KEY 已设置 (长度=${#DEEPSEEK_API_KEY})"
    fi

    # Go 版本检查
    if ! command -v go &>/dev/null; then
        warn "Go 未安装 — 建议安装 Go 1.21+"
        return
    fi
    local go_ver=$(go version 2>/dev/null | grep -oP 'go\K[0-9]+\.[0-9]+' | head -1)
    if [ -n "$go_ver" ]; then
        ok "Go $go_ver"
    else
        log "  Go 版本: ${go_ver:-unknown}，建议 >= 1.21"
    fi

    # Node.js
    if command -v node &>/dev/null; then
        ok "Node.js $(node -v 2>/dev/null), npm $(npm -v 2>/dev/null)"
    else
        fail "Node.js 未安装"
    fi

    # Python3 (封面生成可选)
    if command -v python3 &>/dev/null; then
        ok "python3 $(python3 --version 2>/dev/null | awk '{print $2}')"
    else
        log "  python3 未安装，封面生成将不可用"
    fi

    # --- keys.json ---
    # 作用: AI Provider (L1_AI_Provider) 的 API Key 钱包，管理多个 DeepSeek Key 实现轮转调用
    if [ ! -f "$AP_DIR/config/keys.json" ]; then
        fail "L1_AI_Provider/config/keys.json 不存在"
        echo "       ┌─────────────────────────────────────────────────────────────┐"
        echo "       │  作用: AI Provider 的 DeepSeek API Key 钱包                   │"
        echo "       │  在 L1_AI_Provider/config/ 下创建 keys.json:                  │"
        echo "       │  { \"deepseek\": [\"sk-xxxxxxxx\", \"sk-yyyyyyyy\"] }          │"
        echo "       │  支持多 Key 轮转，提高并发和可用性                              │"
        echo "       └─────────────────────────────────────────────────────────────┘"
    else
        ok "keys.json 已配置"
    fi

    # --- L1_novel_skill/config.json ---
    # 作用: 混元生图 (封面生成) 的腾讯云认证密钥
    if [ ! -f "$SCRIPT_DIR/L1_novel_skill/config.json" ]; then
        log "  L1_novel_skill/config.json 不存在，封面生成将不可用"
        echo "       ┌─────────────────────────────────────────────────────────────┐"
        echo "       │  如需封面生成，请在 L1_novel_skill/ 下创建 config.json:          │"
        echo "       │  {                                                          │"
        echo "       │    \"tencent_secret_id\": \"AKIDxxxxxxxx\",                   │"
        echo "       │    \"tencent_secret_key\": \"xxxxxxxxxxxx\"                   │"
        echo "       │  }                                                          │"
        echo "       │  购买: https://buy.cloud.tencent.com/aiart (混元生图极速版)    │"
        echo "       │  Key 获取: https://console.cloud.tencent.com/cam/capi       │"
        echo "       └─────────────────────────────────────────────────────────────┘"
    else
        ok "L1_novel_skill/config.json 已配置"
    fi
}

# ========== 准备数据目录 ==========
setup_data() {
    log "准备沙箱配置..."
    mkdir -p "$DATA_DIR"
    mkdir -p "$LOG_DIR"
    sudo chown -R admin:admin "$DATA_DIR" 2>/dev/null || true
    sudo chown -R admin:admin "$LOG_DIR" 2>/dev/null || true

    if [ ! -f "$DATA_DIR/opencode_config.json" ]; then
        cat > "$DATA_DIR/opencode_config.json" << 'CONFEOF'
{"permission":{"edit":"allow","bash":"deny","write":"allow","read":"allow","external_directory":"deny","doom_loop":"allow"},"skills":{"paths":["/tmp/sm_demo/skills"]}}
CONFEOF
    fi
    ok "沙箱配置已就绪: $DATA_DIR/opencode_config.json"
}

# ========== 数据库初始化 ==========
init_database() {
    log "初始化数据库..."
    if ! command -v mysql &>/dev/null; then
        log "  mysql 客户端未安装，跳过数据库初始化"
        return
    fi

    local db_user="${MYSQL_ROOT_USER:-root}"
    local db_pass="${MYSQL_ROOT_PASSWORD:-claw123}"
    local db_host="${MYSQL_HOST:-127.0.0.1}"
    local db_port="${MYSQL_PORT:-3306}"

    local mysql_cmd="mysql -h$db_host -P$db_port -u$db_user"
    if [ -n "$db_pass" ]; then
        mysql_cmd="$mysql_cmd -p$db_pass"
    fi

    # 执行 xlongxia schema
    local xlongxia_sql="$SCRIPT_DIR/schema_xlongxia.sql"
    if [ -f "$xlongxia_sql" ]; then
        if $mysql_cmd < "$xlongxia_sql" 2>/tmp/db_err.log; then
            ok "xlongxia 数据库已初始化"
        else
            if grep -qi "already exists\|Access denied" /tmp/db_err.log 2>/dev/null; then
                log "  xlongxia 初始化跳过（数据库已存在或无权限）"
            else
                fail "xlongxia 初始化失败"
                cat /tmp/db_err.log
            fi
        fi
    fi

    # 执行 claw_studios schema
    local clawstudios_sql="$SCRIPT_DIR/schema_claw_studios.sql"
    if [ -f "$clawstudios_sql" ]; then
        if $mysql_cmd < "$clawstudios_sql" 2>/tmp/db_err.log; then
            ok "claw_studios 数据库已初始化"
        else
            if grep -qi "already exists\|Access denied" /tmp/db_err.log 2>/dev/null; then
                log "  claw_studios 初始化跳过（数据库已存在或无权限）"
            else
                fail "claw_studios 初始化失败"
                cat /tmp/db_err.log
            fi
        fi
    fi

    # 执行增量迁移
    if [ -d "$MIGRATIONS_DIR" ]; then
        for f in $(ls "$MIGRATIONS_DIR"/*.sql 2>/dev/null | sort); do
            local fname=$(basename "$f")
            if $mysql_cmd < "$f" 2>/tmp/migrate_err.log; then
                ok "migration: $fname"
            else
                if grep -qi "duplicate column\|already exists\|duplicate key\|Duplicate entry" /tmp/migrate_err.log 2>/dev/null; then
                    log "  migration $fname 已执行过，跳过"
                else
                    fail "migration: $fname"
                    cat /tmp/migrate_err.log
                fi
            fi
        done
    fi
    echo ""
}

# ========== 编译所有模块 ==========
build_all() {
    log "编译所有模块..."
    local build_failed=0

    build_one() {
        local binary="$1" dir="$2" target="$3"
        log "  编译 $binary ..."
        cd "$dir"
        if go build -o "$binary" $target 2>/tmp/build_err.log; then
            ok "$binary"
        else
            fail "$binary"
            cat /tmp/build_err.log
            build_failed=1
        fi
    }

    build_one "skill_registry"   "$SR_DIR"          "."
    build_one "ai_provider"      "$AP_DIR"           "."
    build_one "session_manager"  "$SM_DIR"           "."
    build_one "a1_server"        "$A1_DIR"           "./cmd/a1_server/"
    build_one "workflow_engine"  "$WF_DIR"           "./cmd/workflow_engine/"
    build_one "c2_dashboard"     "$DB_DIR"           "./cmd/c2_dashboard/"
    build_one "scheduler"        "$SCHEDULER_DIR"    "./cmd/scheduler/"
    build_one "bff-server"       "$BFF_DIR"          "."
    build_one "novelcover_pure"  "$COVER_DIR"        "."

    # 编译前端
    log "  编译 Frontend (Next.js) ..."
    cd "$FE_DIR"
    ensure_frontend_permissions
    ensure_frontend_env
    if npm install 2>/tmp/build_err.log && npm run build 2>>/tmp/build_err.log; then
        ok "frontend"
    else
        fail "frontend"
        cat /tmp/build_err.log
        build_failed=1
    fi

    if [ "$build_failed" -ne 0 ]; then
        echo ""
        fail "部分模块编译失败，终止启动"
        exit 1
    fi
    echo ""
}

# ============================================================
# 大模块1: 文档编写 (ailenlu 开发)
# ============================================================

start_skills_register() {
    log "启动 Skills Register (:18090)..."
    cd "$SR_DIR"
    setsid ./skill_registry --port 18090 --internal-auth="" \
        --skill-dir /home/main-repo/L1_skills_register/fixtures \
        --cover-bin "$COVER_DIR/novelcover_pure" \
        --fonts-dir "$COVER_DIR/fonts" \
        > /tmp/sr.log 2>&1 &
    sleep 2
    if curl -s --max-time 3 http://127.0.0.1:18090/api/skill/status > /dev/null 2>&1; then
        ok "Skills Register :18090"
    else
        fail "Skills Register 启动失败"
    fi
}

start_ai_provider() {
    log "启动 AI Provider (:18180)..."
    cd "$AP_DIR"
    setsid ./ai_provider --port 18180 --config-path "$DATA_DIR/opencode_config.json" > /tmp/ap.log 2>&1 &
    sleep 2
    if curl -s --max-time 3 http://127.0.0.1:18180/healthz > /dev/null 2>&1; then
        ok "AI Provider :18180"
    else
        fail "AI Provider 启动失败"
    fi
}

start_session_manager() {
    log "启动 Session Manager (:18080)..."
    cd "$SM_DIR"
    sudo chown -R admin:admin "$DATA_DIR" 2>/dev/null || true
    setsid ./session_manager \
        --port 18080 \
        --data-dir "$DATA_DIR" \
        --max-concurrent 2 \
        --default-timeout-sec 600 \
        --stale-timeout-min 60 \
        --skill-registry http://localhost:18090 \
        > /tmp/sm.log 2>&1 &
    sleep 3
    if curl -s --max-time 3 http://127.0.0.1:18080/api/status > /dev/null 2>&1; then
        ok "Session Manager :18080"
    else
        fail "Session Manager 启动失败"
    fi
}

# ============================================================
# 大模块2: 发布沉淀 (zmp 开发)
# ============================================================

start_a1_vault() {
    log "启动 A1 Account Vault (:8084)..."
    cd "$A1_DIR"
    env \
      A1_DB_DSN="$A1_DB_DSN" \
      A1_ENCRYPTION_KEY="$A1_ENCRYPTION_KEY" \
      A1_MOCK_ENCRYPTION_KEY="$A1_MOCK_ENCRYPTION_KEY" \
      A1_JWT_SECRET="$A1_JWT_SECRET" \
      A1_DB_USER="$A1_DB_USER" \
      A1_DB_PASSWORD="$A1_DB_PASSWORD" \
      A1_DB_HOST="$A1_DB_HOST" \
      A1_DB_NAME="$A1_DB_NAME" \
      setsid ./a1_server > /tmp/a1.log 2>&1 &
    sleep 3
    if curl -s --max-time 3 http://127.0.0.1:8084/healthz > /dev/null 2>&1; then
        ok "A1 Account Vault :8084"
    else
        fail "A1 Account Vault 启动失败"
    fi
}

start_workflow_engine() {
    log "启动 Workflow Engine (:9100)..."
    cd "$WF_DIR"
    setsid ./workflow_engine > /tmp/wf.log 2>&1 &
    sleep 3
    if curl -s --max-time 3 http://127.0.0.1:9100/health > /dev/null 2>&1; then
        ok "Workflow Engine :9100"
    else
        fail "Workflow Engine 启动失败"
    fi
}

# ============================================================
# 大模块3: 定时调度看板 (zmp 开发)
# ============================================================

start_dashboard() {
    log "启动 Dashboard (:8083)..."
    cd "$DB_DIR"
    setsid ./c2_dashboard > /tmp/c2.log 2>&1 &
    sleep 3
    if curl -s --max-time 3 http://127.0.0.1:8083/health > /dev/null 2>&1; then
        ok "Dashboard :8083"
    else
        fail "Dashboard 启动失败"
    fi
}

# ============================================================
# 大模块4: 定时调度器 (Interval Scheduler)
# ============================================================

start_scheduler() {
    log "启动 Interval Scheduler (:9104)..."
    cd "$SCHEDULER_DIR"

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

    setsid env SCHEDULER_CONFIG_PATH="$cfg_path" SCHEDULER_USE_MOCK=true ./scheduler > /tmp/scheduler.log 2>&1 &
    sleep 2
    if curl -s --max-time 3 http://127.0.0.1:9104/healthz > /dev/null 2>&1; then
        ok "Interval Scheduler :9104"
    else
        fail "Interval Scheduler 启动失败"
    fi
}

# ============================================================
# 大模块5: BFF 网关 (zmp 开发)
# ============================================================

start_bff() {
    log "启动 BFF Gateway (:8088)..."
    cd "$BFF_DIR"
    export SESSION_MGR_URL="http://127.0.0.1:18080"
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
    setsid ./bff-server > /tmp/bff.log 2>&1 &
    sleep 2
    if curl -s --max-time 3 http://127.0.0.1:8088/healthz > /dev/null 2>&1; then
        ok "BFF Gateway :8088"
    else
        fail "BFF Gateway 启动失败"
    fi
}

# ========== 前端环境变量（留空则走 Next.js /api、/ws 反代，适配任意 IP） ==========
ensure_frontend_env() {
    local env_file="$FE_DIR/.env.local"
    local desired='# 留空则浏览器请求同源 /api、/ws，由 Next.js 反代到本机 BFF :8088
NEXT_PUBLIC_API_BASE=
NEXT_PUBLIC_WS_BASE=
'

    if [ ! -f "$env_file" ]; then
        echo "$desired" > "$env_file"
        log "  已创建 .env.local（API 走同源 /api 反代）"
        return 0
    fi

    if grep -qE 'NEXT_PUBLIC_(API|WS)_BASE=(http|ws)://' "$env_file" 2>/dev/null; then
        warn "  .env.local 含固定地址（会导致换 IP 部署后请求 localhost/旧 IP），已改为同源反代"
        echo "$desired" > "$env_file"
    fi
}

# ============================================================
# Debug 日志开关
# ============================================================
SM_DEBUG_LOGS="${SM_DEBUG_LOGS:-false}"

# ============================================================
# 大模块6: 前端 (Next.js)
# ============================================================

start_frontend() {
    log "启动 Frontend (:${FE_PORT})..."
    cd "$FE_DIR"
    ensure_frontend_permissions
    ensure_frontend_env

    # 启动前再清一次，避免 build 阶段其他操作重新占用端口
    cleanup_frontend

    local server_ip fe_cmd fe_mode
    server_ip=$(detect_server_ipv4 2>/dev/null || echo "")
    if [ "$server_ip" = "$FRONTEND_DEV_HOST" ]; then
        fe_mode="dev"
        fe_cmd="npm run dev"
        log "  检测到开发机 IP ($server_ip)，使用 npm run dev"
    else
        fe_mode="start"
        fe_cmd="npm run start"
        if [ -n "$server_ip" ]; then
            log "  当前 IP ($server_ip) ≠ $FRONTEND_DEV_HOST，使用 npm run start"
        else
            warn "  未能自动检测 IP，默认使用 npm run start"
        fi
        if [ ! -d ".next" ]; then
            fail "生产模式需要 build 产物，.next 不存在"
            return 1
        fi
    fi

    setsid bash -c "cd '$FE_DIR' && $fe_cmd" > /tmp/fe.log 2>&1 &
    local fe_pid=$!

    if wait_for_frontend_ready "$fe_mode" "$fe_pid"; then
        ok "Frontend :${FE_PORT} ($fe_mode)"
    else
        return 1
    fi
}

# ============================================================
# 主流程
# ============================================================

main() {
    # ---------- 参数解析 ----------
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --debug)
                SM_DEBUG_LOGS=true
                shift
                ;;
            -h|--help)
                echo "用法: bash start_all.sh [--debug]"
                echo ""
                echo "  --debug    启用调试日志，在各任务 CWD 下保存 prompt_debug.log 和 api_trace.jsonl"
                echo "             opencode 进程将输出 DEBUG 级别日志"
                echo "  -h, --help 显示此帮助信息"
                exit 0
                ;;
            *)
                echo "未知参数: $1"
                echo "用法: bash start_all.sh [--debug]"
                exit 1
                ;;
        esac
    done
    # ---------- 参数解析结束 ----------

    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}  全模块后端 + 前端服务启动脚本${NC}"
    echo -e "${CYAN}    运行用户: $RUN_USER${NC}"
    if [ "$SM_DEBUG_LOGS" = "true" ]; then
        echo -e "${YELLOW}    Debug 日志模式已启用${NC}"
    fi
    echo -e "${CYAN}========================================${NC}"
    echo ""

    ensure_run_user

    # 环境安装
    log "=== 环境检测与安装 ==="
    ensure_mysql
    setup_mysql_auth
    ensure_opencode
    echo ""

    cleanup_old
    check_prereq
    setup_data
    init_database
    build_all

    echo ""
    echo -e "${CYAN}--- 大模块1: 文档编写 ---${NC}"
    start_skills_register
    start_ai_provider
    start_session_manager

    echo ""
    echo -e "${CYAN}--- 大模块2: 发布沉淀 ---${NC}"
    start_a1_vault
    start_workflow_engine

    echo ""
    echo -e "${CYAN}--- 大模块3: 调度看板 ---${NC}"
    start_dashboard
    start_scheduler

    echo ""
    echo -e "${CYAN}--- 大模块5: BFF 网关 ---${NC}"
    start_bff

    echo ""
    echo -e "${CYAN}--- 大模块6: 前端 ---${NC}"
    start_frontend

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
}

main
