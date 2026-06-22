#!/usr/bin/env bash
# ============================================================
#  前端生产环境构建 & 启动脚本
#  用法: bash /home/main-repo/frontend_release.sh
#  支持 root 调用（内部自动切换到 admin 执行 npm 命令）
# ============================================================
set -euo pipefail

RUN_USER="admin"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FE_DIR="$SCRIPT_DIR/Front_design"
FE_PORT="${FE_PORT:-3000}"

GREEN='\033[0;32m'
RED='\033[0;31m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${CYAN}[INFO]${NC}  $1"; }
ok()   { echo -e "  ${GREEN}OK${NC}    $1"; }
fail() { echo -e "  ${RED}FAIL${NC}  $1"; }
warn() { echo -e "  ${YELLOW}WARN${NC}  $1"; }

# ========== 以 admin 身份执行 npm 命令（无论当前是 root 还是 admin） ==========
run_as_admin() {
    local current
    current="$(whoami)"
    if [ "$current" = "$RUN_USER" ]; then
        bash -c "$*"
    else
        sudo -u "$RUN_USER" bash -c "$*"
    fi
}

# ========== 修复前端目录权限 ==========
fix_permissions() {
    log "修复前端目录权限（确保属主为 $RUN_USER）..."
    sudo chown -R "$RUN_USER:$RUN_USER" "$FE_DIR" 2>/dev/null && ok "权限修复完成" || warn "chown 失败，继续尝试..."
}

# ========== 确保 .env.local 正确 ==========
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
        warn "  .env.local 含固定地址，已改为同源反代"
        echo "$desired" > "$env_file"
    fi
}

# ========== 清理旧前端进程 ==========
cleanup_frontend() {
    log "清理旧前端进程 (:${FE_PORT})..."

    if command -v fuser &>/dev/null; then
        sudo fuser -k "${FE_PORT}/tcp" 2>/dev/null || true
    elif command -v lsof &>/dev/null; then
        local pids
        pids=$(lsof -t -i ":${FE_PORT}" -sTCP:LISTEN 2>/dev/null || true)
        [ -n "$pids" ] && sudo kill $pids 2>/dev/null || true
    fi

    for pid in $(pgrep -f "npm run start|npm run dev|/bin/next dev|/bin/next start" 2>/dev/null || true); do
        local cwd
        cwd=$(readlink -f "/proc/${pid}/cwd" 2>/dev/null || true)
        if [ "$cwd" = "$FE_DIR" ]; then
            sudo kill "$pid" 2>/dev/null || true
        fi
    done

    sleep 2

    if ss -tlnp 2>/dev/null | grep -q ":${FE_PORT} "; then
        warn "  端口仍被占用，再次释放..."
        command -v fuser &>/dev/null && sudo fuser -k "${FE_PORT}/tcp" 2>/dev/null || true
        sleep 1
    fi

    if ss -tlnp 2>/dev/null | grep -q ":${FE_PORT} "; then
        warn "  端口 :${FE_PORT} 仍被占用，请手动检查"
    else
        ok "前端端口 :${FE_PORT} 已释放"
    fi

    sudo rm -f /tmp/fe.log 2>/dev/null || true
}

# ========== 构建生产包 ==========
build_frontend() {
    log "安装依赖（npm install）..."
    run_as_admin "cd '$FE_DIR' && npm install" 2>/tmp/fe_build.log || {
        fail "npm install 失败，详见 /tmp/fe_build.log"
        cat /tmp/fe_build.log
        exit 1
    }
    ok "依赖安装完成"

    log "构建生产包（npm run build）..."
    run_as_admin "cd '$FE_DIR' && npm run build" >> /tmp/fe_build.log 2>&1 || {
        fail "npm run build 失败，详见 /tmp/fe_build.log"
        cat /tmp/fe_build.log
        exit 1
    }
    ok "生产包构建完成"

    # 构建完再修一次权限，防止 npm 留下 root 属主文件
    sudo chown -R "$RUN_USER:$RUN_USER" "$FE_DIR" 2>/dev/null || true
}

# ========== 等待前端就绪 ==========
wait_for_frontend_ready() {
    local fe_pid="$1"
    local max_wait="${FE_STARTUP_TIMEOUT:-30}"
    local interval=2
    local elapsed=0

    log "  等待 Frontend 就绪（最多 ${max_wait}s）..."

    while [ "$elapsed" -lt "$max_wait" ]; do
        if ! kill -0 "$fe_pid" 2>/dev/null; then
            fail "Frontend 进程已退出（详见 /tmp/fe.log）"
            cat /tmp/fe.log 2>/dev/null || true
            return 1
        fi

        if grep -qE "EADDRINUSE|Failed to compile|Cannot find module" /tmp/fe.log 2>/dev/null; then
            fail "Frontend 启动失败（详见 /tmp/fe.log）"
            cat /tmp/fe.log 2>/dev/null || true
            return 1
        fi

        if grep -qE "Ready|ready|started server|Listening on" /tmp/fe.log 2>/dev/null; then
            if curl -s -o /dev/null --max-time 5 "http://127.0.0.1:${FE_PORT}/" 2>/dev/null; then
                return 0
            fi
        fi

        sleep "$interval"
        elapsed=$((elapsed + interval))
    done

    fail "Frontend 启动超时（${max_wait}s），详见 /tmp/fe.log"
    return 1
}

# ========== 启动生产前端 ==========
start_frontend() {
    log "启动前端生产服务（npm run start）..."

    if [ ! -d "$FE_DIR/.next" ]; then
        fail ".next 不存在，请先执行构建"
        exit 1
    fi

    setsid sudo -u "$RUN_USER" bash -c "cd '$FE_DIR' && npm run start" > /tmp/fe.log 2>&1 &
    local fe_pid=$!

    if wait_for_frontend_ready "$fe_pid"; then
        ok "Frontend :${FE_PORT} (start) — pid=$fe_pid"
        ok "日志: /tmp/fe.log"
    else
        exit 1
    fi
}

# ============================================================
#  主流程
# ============================================================
main() {
    echo ""
    echo -e "${CYAN}============================================${NC}"
    echo -e "${CYAN}  前端生产环境构建 & 启动${NC}"
    echo -e "${CYAN}============================================${NC}"
    echo ""

    fix_permissions
    ensure_frontend_env
    cleanup_frontend
    build_frontend
    start_frontend

    echo ""
    echo -e "${GREEN}前端启动完成：http://localhost:${FE_PORT}${NC}"
    echo ""
}

main
