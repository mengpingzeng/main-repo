#!/usr/bin/env bash
# ============================================================
# batch-create-shadows.sh — 批量将公版小说 .txt 转化为 shadow 技能
#
# 依赖: opencode (AI驱动), python3, L1_novel_skill/generate_cover.py
# 共享库: shadow_utils.sh (同目录)
# ============================================================
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/shadow_utils.sh"

# ---- 默认值 ----
OUTPUT_DIR_DEFAULT="./shadow_output"
TIMEOUT_DEFAULT=2400
LIMIT=""  # 空 = 处理全部; 数字 = 最多处理 N 部新小说（已跳过的跳过的不计入）
MODEL="${SHADOW_OPENCODE_MODEL:-}"
OPENAIDE_DIR="${SHADOW_OPENAIDE_DIR:-/home/main-repo}"
GENERATE_COVER_SCRIPT="${SHADOW_COVER_SCRIPT:-$SCRIPT_DIR/../L1_novel_skill/scripts/generate_cover.py}"

# ---- 帮助 ----
show_help() {
    cat << 'EOF'
╔══════════════════════════════════════════════════════════════╗
║       batch-create-shadows — 批量 txt → shadow 技能         ║
╚══════════════════════════════════════════════════════════════╝

【功能】
  将一批公版小说 .txt 文件，逐个通过 AI（opencode）转化为
  shadow 写作技能目录。每部小说独立处理，互不干扰。

【基本用法】
  ./batch-create-shadows --list novels.txt

【输入文件格式】
  novels.txt 中每行一个小说 .txt 的绝对路径:
    /data/novels/Dracula.txt
    /data/novels/Frankenstein.txt
    /data/novels/Jane_Eyre.txt
    # 以 # 开头的行为注释，空行自动跳过

【选项】
  --list FILE          小说列表文件（每行一个 .txt 路径）[必填]
  --output-dir DIR     输出目录（默认: ./shadow_output）
  --timeout SECONDS    单部小说处理超时秒数（默认: 2400 = 40分钟）
  --limit N             最多处理 N 部未完成的小说（已完成的跳过不计入）
  --model PROVIDER/MODEL  指定 opencode 使用的模型
   --retry-cover-only   仅重试封面生成失败的条目（不重新跑 AI）
  --no-cover           跳过封面生成
  --help               显示本帮助

【工作流程（每部小说）】
  1. 调用 opencode run → AI 执行 4 阶段工作流:
     Phase A: 原作消化 (分析统计 + 风格指纹 + 内核提炼)
     Phase B: 创作决策 (大纲 + 角色 + 背景)
     Phase C: Skill 物化 (12 个文件产出)
     Phase D: 交付
  2. 自动校验 13 项产出文件
3. 若缺 cover.png → 自动调用 generate_cover.py 重试生图
4. 自动生成 _meta.json

【重试】
   脚本自动跳过已完成的 shadow 目录，中断后直接重新运行即可。
   仅重试失败的封面:
     ./batch-create-shadows --list novels.txt --retry-cover-only

【环境变量】
  SHADOW_OPENCODE_MODEL    指定模型 (如 deepseek/deepseek-v4-pro)
  SHADOW_OPENAIDE_DIR      opencode 工作目录 (默认 /home/main-repo)
  SHADOW_COVER_SCRIPT      封面生成脚本路径
  SHADOW_OWNER_ID          _meta.json 中的 ownerId
  FORCE_COLOR=1            强制彩色输出

【完整示例】
  # 准备小说列表
  find /data/public_domain -name '*.txt' > novels.txt

   # 批量生成（首次运行，中断后直接重跑即可，会自动跳过已完成条目）
   ./batch-create-shadows --list novels.txt --output-dir ./my_shadows

   # 封面生成失败的？单独重试
   ./batch-create-shadows --list novels.txt --retry-cover-only

  # 全部生成完后，注册到 skill_registry
  ls -d ./my_shadows/*-shadow/ > to_register.txt
  ./register-shadow --batch to_register.txt
EOF
}

# ---- 封面生成 ----
retry_cover() {
    local dir="$1"
    local prompt cover_output

    prompt=$(_novel_cover_prompt "$dir")
    if [ -z "$prompt" ]; then
        warn "novel_metadata.json 中无 cover_prompt，无法自动生成封面"
        warn "请手动执行: python3 $GENERATE_COVER_SCRIPT --prompt '...' --output $dir/cover.png"
        return 1
    fi

    cover_output="$dir/cover.png"
    info "重试封面生成..."
    info "  Prompt: ${prompt:0:120}..."

    if [ ! -f "$GENERATE_COVER_SCRIPT" ]; then
        warn "封面生成脚本不存在: $GENERATE_COVER_SCRIPT"
        return 1
    fi

    python3 "$GENERATE_COVER_SCRIPT" --prompt "$prompt" --output "$cover_output" 2>&1 | while IFS= read -r line; do
        echo "        $line"
    done

    if [ -f "$cover_output" ] && [ "$(file_size "$cover_output")" -gt 10240 ]; then
        ok "封面生成成功 ($(human_size $(file_size "$cover_output")))"
        # 更新 novel_metadata.json 中的封面字段
        python3 -c "
import json, os
f='$dir/novel_metadata.json'
if os.path.exists(f):
    d=json.load(open(f))
    d['cover_image']='./cover.png'
    d['cover_generated_by']='混元 TextToImageLite'
    d['cover_resolution']='768x1024 (3:4)'
    json.dump(d, open(f,'w'), ensure_ascii=False, indent=2)
    open(f,'a').write('\n')
" 2>/dev/null
        return 0
    else
        fail "封面生成失败"
        return 1
    fi
}

# ---- 单部小说处理 ----
process_novel() {
    local novel_path="$1" output_dir="$2" no_cover="$3"

    local novel_name=$(basename "$novel_path" .txt)
    local log_file="$output_dir/logs/opencode.log"

    heading "处理: $novel_name"
    info "源文件: $novel_path"
    info "输出目录: $output_dir"

    # 创建输出目录和日志目录
    mkdir -p "$output_dir/logs"

    # 写入运行信息头
    {
        echo "=== opencode run started at $(date -Iseconds) ==="
        echo "Novel: $novel_path"
        echo "Output: $output_dir"
        echo "Model: ${MODEL:-default}"
        echo "Timeout: ${TIMEOUT}s"
        echo ""
    } >> "$log_file"

    # 构建 opencode prompt
    local prompt="使用 novel-shadow-creator 技能处理公版小说。

源小说文件路径: $novel_path
输出目录: $output_dir

严格按照skill定义的4阶段工作流执行:
  Phase A: 原作消化 (统计分析 + 风格指纹 + 内核提炼)
  Phase B: 创作决策 (大纲 + 角色 + 背景，自动决策不需询问)
  Phase C: Skill物化 (必须生成全部12个文件)
  Phase D: 交付

重要要求:
1. novel_metadata.json 必须包含 cover_prompt 字段（用于后续封面生成）
2. 产出目录结构严格遵循skill规范
3. 完成后无需额外说明，直接退出"

    if [ "$no_cover" = "true" ]; then
        prompt="$prompt

4. 封面生成已禁用，跳过 cover.png 生成"
    fi

    # 调用 opencode
    info "启动 opencode (日志: $log_file)..."
    local opencode_args=(run --dir "$OPENAIDE_DIR")
    [ -n "$MODEL" ] && opencode_args+=(--model "$MODEL")
    opencode_args+=(--dangerously-skip-permissions)
    opencode_args+=("$prompt")
    opencode_args+=(--file "$novel_path")

    local start_time=$(date +%s)
    local opencode_rc=0

    # 使用 timeout 包裹，输出同时到终端和日志文件
    # 日志文件通过 perl 剥离 ANSI 转义码，终端保持原样
    set +o pipefail
    timeout --kill-after=10 "$TIMEOUT" opencode "${opencode_args[@]}" 2>&1 </dev/null \
        | tee -a "$log_file"
    opencode_rc=${PIPESTATUS[0]}
    set -o pipefail
    # 剥离日志文件中的 ANSI 转义码
    if [ -f "$log_file" ]; then
        perl -i -pe 's/\x1b\[[0-9;]*[a-zA-Z]//g' "$log_file"
    fi

    local elapsed=$(($(date +%s) - start_time))

    {
        echo ""
        echo "=== opencode run finished at $(date -Iseconds) ==="
        echo "Exit code: $opencode_rc"
        echo "Elapsed: ${elapsed}s"
    } >> "$log_file"

    info "opencode 退出码: $opencode_rc (耗时 ${elapsed}s)"

    if [ $opencode_rc -eq 124 ]; then
        fail "超时 (${TIMEOUT}s)，opencode 进程已终止"
        # 确保彻底清理
        pkill -f "opencode run" 2>/dev/null || true
        sleep 1
        return 2
    fi

    return 0
}

# ---- 主逻辑 ----
main() {
    local LIST_FILE="" OUTPUT_DIR="$OUTPUT_DIR_DEFAULT"
    local TIMEOUT="$TIMEOUT_DEFAULT" RETRY_COVER_ONLY=false NO_COVER=false
    local mode="full"

    # 解析参数
    while [ $# -gt 0 ]; do
        case "$1" in
            --help|-h)   show_help; exit 0 ;;
            --list)      LIST_FILE="$2"; shift 2 ;;
            --output-dir) OUTPUT_DIR="$2"; shift 2 ;;
            --timeout)   TIMEOUT="$2"; shift 2 ;;
            --limit)     LIMIT="$2"; shift 2 ;;
            --model)     MODEL="$2"; shift 2 ;;
            --retry-cover-only) RETRY_COVER_ONLY=true; shift ;;
            --no-cover)  NO_COVER=true; shift ;;
            *) fail "未知参数: $1 (用 --help 查看帮助)"; exit 1 ;;
        esac
    done

    # 校验
    [ -n "$LIST_FILE" ] || { fail "缺少 --list <文件> (用 --help 查看帮助)"; exit 1; }
    [ -f "$LIST_FILE" ] || die "列表文件不存在: $LIST_FILE"
    check_deps || exit 1
    command -v opencode &>/dev/null || die "opencode 未安装或不在 PATH 中"

    if [ "$NO_COVER" = false ] && [ ! -f "$GENERATE_COVER_SCRIPT" ]; then
        warn "封面生成脚本不存在: $GENERATE_COVER_SCRIPT"
        warn "封面生成将不可用。设置 SHADOW_COVER_SCRIPT 环境变量指定路径。"
    fi

    mkdir -p "$OUTPUT_DIR"
    OUTPUT_DIR="$(cd "$OUTPUT_DIR" && pwd)"

    # 读列表
    local novels=()
    while IFS= read -r line; do
        line="${line//$'\r'/}"
        [[ -z "$line" ]] && continue
        [[ "$line" =~ ^[[:space:]]*# ]] && continue
        [ -f "$line" ] && novels+=("$line") || warn "跳过不存在的文件: $line"
    done < "$LIST_FILE"

    [ ${#novels[@]} -gt 0 ] || die "列表中没有找到有效的小说文件"

    # 校验 --limit
    if [ -n "$LIMIT" ]; then
        if ! [[ "$LIMIT" =~ ^[0-9]+$ ]] || [ "$LIMIT" -le 0 ]; then
            fail "--limit 必须为正整数"
            exit 1
        fi
    fi

    heading "批量生成 shadow 技能"
    info "共 ${#novels[@]} 部小说"
    info "输出目录: $OUTPUT_DIR"
    info "单部超时: ${TIMEOUT}s"
    [ -n "$LIMIT" ] && info "处理上限: 最多 ${LIMIT} 部（已完成跳过不计入）"
    [ -n "$MODEL" ] && info "模型: $MODEL"
    [ "$RETRY_COVER_ONLY" = true ] && info "模式: 仅封面重试"
    [ "$NO_COVER" = true ] && warn "封面生成已禁用"

    local total=${#novels[@]}
    local done_count=0 skip_count=0 fail_count=0 cover_fail=0
    local processed_count=0  # 实际处理的部数（成功+失败，跳过的不计）
    local start_all=$(date +%s)

    for novel in "${novels[@]}"; do
        local novel_name=$(basename "$novel" .txt)
        # 规范化目录名: 转小写, 空格→短横, 去特殊字符, 加 -shadow
        local slug=$(slug_from_path "$novel")
        local shadow_dir="$OUTPUT_DIR/${slug}-shadow"

        echo ""

        # ---- 仅封面重试 ----
        if [ "$RETRY_COVER_ONLY" = true ]; then
            if [ ! -d "$shadow_dir" ] || ! validate_except_cover "$shadow_dir"; then
                skip "$novel_name (shadow目录不存在或核心文件不完整)"
                ((skip_count++)) || true
                continue
            fi
            if [ -f "$shadow_dir/cover.png" ] && [ "$(file_size "$shadow_dir/cover.png")" -gt 10240 ]; then
                skip "$novel_name (封面已正常)"
                ((skip_count++)) || true
                continue
            fi
            info "重试封面: $novel_name"
            if retry_cover "$shadow_dir"; then
                ok "$novel_name (封面已修复)"
                ((done_count++)) || true
            else
                fail "$novel_name (封面重试失败)"
                ((cover_fail++)) || true
            fi
            ((processed_count++)) || true
            if [ -n "$LIMIT" ] && [ "$processed_count" -ge "$LIMIT" ]; then
                info "已达处理上限 (${LIMIT}部)，停止"
                break
            fi
            continue
        fi

        # ---- 完整处理 ----
        [ -f "$novel" ] || {
            fail "$novel_name: 源文件已不存在"
            ((fail_count++)) || true
            ((processed_count++)) || true
            if [ -n "$LIMIT" ] && [ "$processed_count" -ge "$LIMIT" ]; then
                info "已达处理上限 (${LIMIT}部)，停止"
                break
            fi
            continue
        }

        # 检查 shadow 目录
        if [ -d "$shadow_dir" ]; then
            if validate_quiet "$shadow_dir"; then
                skip "$novel_name (shadow 目录已存在且完整)"
                ((skip_count++)) || true
                continue
            else
                warn "$novel_name: shadow 目录存在但不完整, 将重新生成"
            fi
        fi

        # 调用 opencode
        process_novel "$novel" "$shadow_dir" "$NO_COVER"
        local process_rc=$?

        # 生成 _meta.json (如果缺失) — 必须在校验之前
        if [ ! -f "$shadow_dir/_meta.json" ]; then
            generate_meta_json "$shadow_dir" "${slug}-shadow"
        fi

        # 校验产出
        local validation_ok=true
        local cover_ok=true

        if ! validate_quiet "$shadow_dir"; then
            warn "产出校验未完全通过:"
            validate_shadow_dir "$shadow_dir" >/dev/null || true
            validation_ok=false
        fi

        # 检查封面
        if [ "$NO_COVER" = false ]; then
            if [ ! -f "$shadow_dir/cover.png" ] || [ "$(file_size "$shadow_dir/cover.png")" -le 10240 ]; then
                # 尝试封面重试
                if [ -f "$GENERATE_COVER_SCRIPT" ]; then
                    info "封面缺失, 尝试自动生成..."
                    if retry_cover "$shadow_dir"; then
                        validation_ok=true
                    else
                        cover_ok=false
                    fi
                else
                    cover_ok=false
                fi
            fi
        fi

        # 最终状态
        if [ "$validation_ok" = true ] && [ "$cover_ok" = true ]; then
            ok "$novel_name → $(basename "$shadow_dir")"
            ((done_count++)) || true
        elif [ "$validation_ok" = true ] && [ "$cover_ok" = false ]; then
            warn "$novel_name (封面生成失败, 可用 --retry-cover-only 重试)"
            ((cover_fail++)) || true
        else
            fail "$novel_name (校验未通过)"
            ((fail_count++)) || true
        fi

        # 成功或失败都算已处理（跳过的不计）
        ((processed_count++)) || true
        if [ -n "$LIMIT" ] && [ "$processed_count" -ge "$LIMIT" ]; then
            info "已达处理上限 (${LIMIT}部)，停止"
            break
        fi

    done

    local total_elapsed=$(($(date +%s) - start_all))

    # 输出统计
    echo ""
    heading "批量生成完成"
    echo "  总数: ${total}"
    echo "  成功: ${C_GREEN}${done_count}${C_RESET}"
    [ "$skip_count" -gt 0 ] && echo "  跳过: ${C_YELLOW}${skip_count}${C_RESET}"
    [ "$cover_fail" -gt 0 ] && echo "  封面失败: ${C_YELLOW}${cover_fail}${C_RESET}"
    [ "$fail_count" -gt 0 ] && echo "  失败: ${C_RED}${fail_count}${C_RESET}"
    echo "  耗时: ${total_elapsed}s"
    if [ -n "$LIMIT" ] && [ "$processed_count" -ge "$LIMIT" ]; then
        echo "  (已达处理上限 ${LIMIT} 部，提前停止；列表其余条目未处理)"
    fi
    echo ""
    if [ "$cover_fail" -gt 0 ]; then
        tip "封面失败的条目, 可运行: ./batch-create-shadows --list $LIST_FILE --retry-cover-only"
    fi
    if [ "$done_count" -gt 0 ]; then
        tip "生成完成的 shadow 位于: $OUTPUT_DIR"
        tip "注册到 skill_registry: ls -d $OUTPUT_DIR/*-shadow/ > list.txt && ./register-shadow --batch list.txt"
    fi
}

main "$@"
