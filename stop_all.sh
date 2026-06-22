#!/usr/bin/env bash
# ============================================================
#  全模块后端 + 前端服务停止脚本
#  用法: bash stop_all.sh
# ============================================================
set -euo pipefail

# ========== 路径常量（与 start_all.sh 保持一致） ==========
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FE_DIR="$SCRIPT_DIR/Front_design"
FE_PORT="${FE_PORT:-3000}"

# ========== 颜色 ==========
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[0;33m'
NC='\033[0m'

log()  { echo -e "${CYAN}[INFO]${NC}  $1"; }
ok()   { echo -e "  ${GREEN}OK${NC}    $1"; }
warn() { echo -e "  ${YELLOW}WARN${NC}  $1"; }

stopped_count=0

# ========== 杀掉匹配进程 ==========
kill_proc() {
    local desc="$1" pattern="$2"
    local pids
    pids=$(pgrep -f "$pattern" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        log "停止 $desc ... (PIDs: $(echo $pids | tr '\n' ' '))"
        sudo pkill -f "$pattern" 2>/dev/null || true
        sleep 0.5
        # 如果还没死，强杀
        local leftovers
        leftovers=$(pgrep -f "$pattern" 2>/dev/null || true)
        if [ -n "$leftovers" ]; then
            warn "$desc 未响应，强制终止..."
            sudo pkill -9 -f "$pattern" 2>/dev/null || true
        fi
        ok "$desc 已停止"
        stopped_count=$((stopped_count + 1))
    else
        warn "$desc 未在运行"
    fi
}

# ========== 按端口杀掉占用进程 ==========
kill_port() {
    local desc="$1" port="$2"
    local pids
    pids=$(sudo lsof -ti tcp:"$port" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        log "停止 $desc (端口 $port) ... (PIDs: $(echo $pids | tr '\n' ' '))"
        sudo kill -15 $pids 2>/dev/null || true
        sleep 0.5
        local leftovers
        leftovers=$(sudo lsof -ti tcp:"$port" 2>/dev/null || true)
        if [ -n "$leftovers" ]; then
            warn "$desc (端口 $port) 未响应，强制终止..."
            sudo kill -9 $leftovers 2>/dev/null || true
        fi
        ok "$desc (端口 $port) 已释放"
        stopped_count=$((stopped_count + 1))
    else
        warn "端口 $port 未被占用"
    fi
}

# ========== 停止前端（覆盖 dev/start 两种模式及 Next.js 16 next-server） ==========
stop_frontend() {
    local port="${FE_PORT}"

    # 1. 按端口释放（:3000）
    local pids
    pids=$(sudo lsof -ti tcp:"$port" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        log "停止 Frontend (端口 :$port) ... (PIDs: $(echo $pids | tr '\n' ' '))"
        sudo kill -15 $pids 2>/dev/null || true
        sleep 0.5
        local leftovers
        leftovers=$(sudo lsof -ti tcp:"$port" 2>/dev/null || true)
        if [ -n "$leftovers" ]; then
            warn "Frontend (端口 :$port) 未响应，强制终止..."
            sudo kill -9 $leftovers 2>/dev/null || true
        fi
        ok "Frontend (端口 :$port) 已释放"
        stopped_count=$((stopped_count + 1))
    else
        warn "端口 :$port 未被占用"
    fi

    # 2. 按进程名清理（dev/start 模式 + Next.js 16 的 next-server）
    for pattern in "next dev" "next start" "next-server"; do
        local fpids
        fpids=$(pgrep -f "$pattern" 2>/dev/null || true)
        if [ -n "$fpids" ]; then
            log "停止 Frontend ($pattern) ... (PIDs: $(echo $fpids | tr '\n' ' '))"
            sudo pkill -f "$pattern" 2>/dev/null || true
            sleep 0.3
            local leftovers
            leftovers=$(pgrep -f "$pattern" 2>/dev/null || true)
            if [ -n "$leftovers" ]; then
                warn "Frontend ($pattern) 未响应，强制终止..."
                sudo pkill -9 -f "$pattern" 2>/dev/null || true
            fi
        fi
    done

    # 3. 清理 Front_design 目录下残留的 npm / next 包装进程
    for pid in $(pgrep -f "npm run start|npm run dev|/bin/next dev|/bin/next start" 2>/dev/null || true); do
        local cwd
        cwd=$(readlink -f "/proc/${pid}/cwd" 2>/dev/null || true)
        if [ "$cwd" = "$FE_DIR" ]; then
            sudo kill "$pid" 2>/dev/null || true
        fi
    done

    sleep 0.5
}

# ============================================================
# 主流程
# ============================================================

main() {
    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}  全模块后端 + 前端服务停止脚本${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""

    # --- 按进程名停止 ---
    log "正在按进程名停止服务..."
    kill_proc "BFF Gateway (bff-server)"     "bff-server"
    kill_proc "Interval Scheduler"           "scheduler"
    kill_proc "Dashboard"                    "c2_dashboard"
    kill_proc "Workflow Engine"              "workflow_engine"
    kill_proc "A1 Account Vault"             "a1_server"
    kill_proc "Session Manager"              "session_manager"
    kill_proc "AI Provider"                  "ai_provider"
    kill_proc "Skills Register"              "skill_registry"
    kill_proc "BFF run_bff.sh"               "run_bff.sh"
    echo ""

    # --- 停止前端（端口 + 进程名 + 目录校验三管齐下） ---
    log "正在停止前端服务..."
    stop_frontend
    echo ""

    # --- 按端口兜底清理 ---
    log "正在按端口清理残留..."
    kill_port "BFF Gateway"          "8088"
    kill_port "Interval Scheduler"   "9104"
    kill_port "Dashboard"            "8083"
    kill_port "A1 Account Vault"     "8084"
    kill_port "Workflow Engine"      "9100"
    kill_port "Session Manager"      "18080"
    kill_port "Skills Register"      "18090"
    kill_port "AI Provider"          "18180"
    echo ""

    # --- 清理残留的 defunct/僵尸子进程 ---
    # session_manager 在第 140 行已被停止，其子 opencode 进程会自动终止
    # 不再使用 pkill -f "opencode" 通杀，避免误杀其他独立运行的 opencode 会话

    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${GREEN}所有服务已停止${NC}"
    echo ""
}

main
