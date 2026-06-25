#!/usr/bin/env bash
# ============================================================
# shadow_utils.sh — Shadow 技能管理共享函数库
# 被 batch-create-shadows / register-shadow / unregister-shadow 共用
# ============================================================

# ---- 颜色输出 ----
if [ -t 1 ] || [ -n "${FORCE_COLOR:-}" ]; then
    C_GREEN='\033[0;32m'
    C_YELLOW='\033[1;33m'
    C_RED='\033[0;31m'
    C_BLUE='\033[0;34m'
    C_CYAN='\033[0;36m'
    C_BOLD='\033[1m'
    C_RESET='\033[0m'
    C_DIM='\033[2m'
else
    C_GREEN='' C_YELLOW='' C_RED='' C_BLUE='' C_CYAN='' C_BOLD='' C_RESET='' C_DIM=''
fi

ok()   { echo -e "${C_GREEN}[OK]${C_RESET} $*" >&2; }
skip() { echo -e "${C_YELLOW}[SKIP]${C_RESET} $*" >&2; }
fail() { echo -e "${C_RED}[FAIL]${C_RESET} $*" >&2; }
warn() { echo -e "${C_YELLOW}[WARN]${C_RESET} $*" >&2; }
info() { echo -e "${C_BLUE}[ ..]${C_RESET} $*" >&2; }
tip()  { echo -e "${C_CYAN}[TIP]${C_RESET} $*" >&2; }
die()  { echo -e "${C_RED}[FATAL]${C_RESET} $*" >&2; exit 1; }

heading() {
    echo "" >&2
    echo -e "${C_BOLD}$*${C_RESET}" >&2
    echo -e "${C_DIM}───────────────────────────────────────────────${C_RESET}" >&2
}

# ---- 路径 ----
UTIL_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_FIXTURES_DIR="$UTIL_SCRIPT_DIR/fixtures"

slug_from_path() {
    local path="$1"
    local base
    base=$(basename "$path" .txt)
    base=$(python3 -c "
import re, sys
s = sys.argv[1].strip()
s = s.lower()
s = re.sub(r'\s+', '-', s)
# Keep Unicode word chars and hyphens; replace everything else
s = re.sub(r'[^\w\-]', '-', s, flags=re.UNICODE)
s = re.sub(r'-{2,}', '-', s)
s = s.strip('-')
print(s if s else 'unnamed')
" "$base")
    echo "$base"
}

get_fixtures_dir() { echo "${SHADOW_FIXTURES_DIR:-$DEFAULT_FIXTURES_DIR}"; }

# ---- JSON 读取 (python3 内置, 不依赖 jq) ----
_json_field() {
    local file="$1" key="$2"
    python3 -c "
import json, sys
try:
    with open('$file','r') as f:
        data = json.load(f)
    for k in '$key'.split('.'):
        data = data.get(k, '') if isinstance(data, dict) else ''
    if isinstance(data, str):
        print(data)
    elif isinstance(data, (int, float)):
        print(data)
    else:
        print(json.dumps(data, ensure_ascii=False) if data else '')
except Exception:
    pass
" 2>/dev/null
}

_meta_slug()    { _json_field "$1/_meta.json" slug; }
_meta_version() { _json_field "$1/_meta.json" version; }
_novel_title()  { _json_field "$1/novel_metadata.json" title; }
_novel_cover_prompt() { _json_field "$1/novel_metadata.json" cover_prompt; }

# 统计 chapters/ 下 .md 文件数
chapter_count() {
    local d="$1/chapters"
    if [ -d "$d" ]; then
        find "$d" -maxdepth 1 -name 'chapter_*.md' 2>/dev/null | wc -l
    else
        echo 0
    fi
}

# 人性化文件大小
human_size() {
    local b=$1
    if [ "$b" -lt 1024 ]; then echo "${b} B"
    elif [ "$b" -lt 1048576 ]; then echo "$((b / 1024)) KB"
    else echo "$((b / 1048576)) MB"
    fi
}

file_size() {
    stat -c%s "$1" 2>/dev/null || stat -f%z "$1" 2>/dev/null || echo 0
}

dir_total_size() {
    du -sb "$1" 2>/dev/null | cut -f1 || echo 0
}

# ---- 依赖检查 ----
check_deps() {
    local missing=()
    for cmd in python3; do
        command -v "$cmd" &>/dev/null || missing+=("$cmd")
    done
    if [ ${#missing[@]} -gt 0 ]; then
        fail "缺少依赖: ${missing[*]}"
        return 1
    fi
    return 0
}

# ---- 校验 shadow 目录 (13项) ----
# 返回值: 0=全部通过, 1=有错误, 2=仅有警告
validate_shadow_dir() {
    local dir="$1"
    local errors=0 warnings=0

    [ -d "$dir" ] || { fail "目录不存在: $dir"; return 1; }
    local name=$(basename "$dir")

    # 1-8: 核心文件
    for f in SKILL.md README.md style_fingerprint.yaml outline.json \
             state.json summaries.md chapter_prompt.md self_check.md; do
        if [ -f "$dir/$f" ] && [ "$(file_size "$dir/$f")" -gt 0 ]; then
            ok "  $f"
        else
            fail "  $f"
            ((errors++))
        fi
    done

    # 9. novel_metadata.json
    if [ -f "$dir/novel_metadata.json" ]; then
        local t=$(_novel_title "$dir")
        if [ -n "$t" ]; then
            ok "  novel_metadata.json  ($t)"
        else
            warn "  novel_metadata.json  (title 为空)"
            ((warnings++))
        fi
    else
        fail "  novel_metadata.json"
        ((errors++))
    fi

    # 10. chapters/
    [ -d "$dir/chapters" ] && ok "  chapters/" || { fail "  chapters/"; ((errors++)); }

    # 11. scripts/
    if [ -d "$dir/scripts" ]; then
        if [ -f "$dir/scripts/similarity_check.py" ] && [ -f "$dir/scripts/state_machine.py" ]; then
            ok "  scripts/ (完整)"
        else
            fail "  scripts/ (缺少 .py)"
            ((errors++))
        fi
    else
        fail "  scripts/"
        ((errors++))
    fi

    # 12. cover.png
    if [ -f "$dir/cover.png" ]; then
        local cs=$(file_size "$dir/cover.png")
        if [ "$cs" -gt 10240 ]; then
            ok "  cover.png    ($(human_size $cs))"
        else
            warn "  cover.png    (过小: $(human_size $cs))"
            ((warnings++))
        fi
    else
        fail "  cover.png"
        ((errors++))
    fi

    # 13. _meta.json
    if [ -f "$dir/_meta.json" ]; then
        local s=$(_meta_slug "$dir")
        local v=$(_meta_version "$dir")
        if [ -n "$s" ] && [ -n "$v" ]; then
            ok "  _meta.json   (slug=$s v$v)"
        else
            fail "  _meta.json   (缺 slug/version)"
            ((errors++))
        fi
    else
        fail "  _meta.json"
        ((errors++))
    fi

    [ $errors -eq 0 ] && [ $warnings -eq 0 ] && return 0
    [ $errors -eq 0 ] && return 2
    return 1
}

# 快速校验 (只返回通过/不通过, 不打印详情)
validate_quiet() {
    local dir="$1" errors=0
    [ -d "$dir" ] || return 1
    local files=(
        SKILL.md README.md style_fingerprint.yaml outline.json
        state.json summaries.md chapter_prompt.md self_check.md
        novel_metadata.json
    )
    for f in "${files[@]}"; do
        [ -f "$dir/$f" ] && [ "$(file_size "$dir/$f")" -gt 0 ] || ((errors++))
    done
    [ -d "$dir/chapters" ] || ((errors++))
    [ -d "$dir/scripts" ] || ((errors++))
    [ -f "$dir/scripts/similarity_check.py" ] || ((errors++))
    [ -f "$dir/scripts/state_machine.py" ] || ((errors++))
    [ -f "$dir/cover.png" ] && [ "$(file_size "$dir/cover.png")" -gt 10240 ] || ((errors++))
    [ -f "$dir/_meta.json" ] || ((errors++))
    [ "$errors" -eq 0 ]
}

# 快速校验 (排除cover.png, 用于 retry-cover-only 模式判定)
validate_except_cover() {
    local dir="$1" errors=0
    [ -d "$dir" ] || return 1
    local files=(
        SKILL.md README.md style_fingerprint.yaml outline.json
        state.json summaries.md chapter_prompt.md self_check.md
        novel_metadata.json
    )
    for f in "${files[@]}"; do
        [ -f "$dir/$f" ] && [ "$(file_size "$dir/$f")" -gt 0 ] || ((errors++))
    done
    [ -d "$dir/chapters" ] || ((errors++))
    [ -d "$dir/scripts" ] || ((errors++))
    [ -f "$dir/scripts/similarity_check.py" ] || ((errors++))
    [ -f "$dir/scripts/state_machine.py" ] || ((errors++))
    [ -f "$dir/_meta.json" ] || ((errors++))
    [ "$errors" -eq 0 ]
}

# 生成 _meta.json
generate_meta_json() {
    local dir="$1" slug="$2" version="${3:-1.0.0}"
    local ownerId="${SHADOW_OWNER_ID:-kn71a7me3jfssnxv4fnmxv673n82geeb}"
    local ts=$(date +%s)
    python3 -c "
import json
meta = {
    'ownerId': '$ownerId',
    'slug': '$slug',
    'version': '$version',
    'publishedAt': $ts
}
with open('$dir/_meta.json', 'w') as f:
    json.dump(meta, f, indent=2)
    f.write('\n')
"
    ok "_meta.json 已生成  (slug=$slug v$version)"
}

# ---- 确认对话框 ----
confirm() {
    local prompt="${1:-确认? (y/N): }"
    local reply
    read -r -p "$prompt" reply
    case "$reply" in
        [yY]|[yY][eE][sS]) return 0 ;;
        *) return 1 ;;
    esac
}

confirm_delete_all() {
    local count="$1"
    local prompt="${2:-确认删除全部 ${count} 个 shadow? 输入 \"DELETE ${count}\" 确认: }"
    local reply
    read -r -p "$prompt" reply
    [ "$reply" = "DELETE ${count}" ] || [ "$reply" = "DELETE ALL" ]
}

# ---- 列表文件解析 ----
# 读取列表文件, 跳过空行和 # 注释
parse_list_file() {
    local file="$1"
    [ -f "$file" ] || { fail "列表文件不存在: $file"; return 1; }
    local count=0
    while IFS= read -r line || [ -n "$line" ]; do
        line="${line//$'\r'/}"
        [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
        echo "$line"
        ((count++))
    done < "$file"
    return 0
}

# ---- 超时安全执行 ----
run_with_timeout() {
    local secs="$1"; shift
    timeout --kill-after=10 "$secs" "$@"
    local rc=$?
    if [ $rc -eq 124 ]; then
        warn "超时 (${secs}s) — 已强制终止"
    fi
    return $rc
}
