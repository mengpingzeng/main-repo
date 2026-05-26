import type { Model } from "@/types"

/** 由 /api/models 构建 id → 展示名 映射 */
export function buildModelNameMap(models: Model[]): Record<string, string> {
  const map: Record<string, string> = {}
  for (const m of models) {
    if (m.id) map[m.id] = m.name || m.id
  }
  return map
}

/** 将任务存储的 model id 转为与新建任务页一致的展示名 */
export function formatModelName(modelId: string, nameMap?: Record<string, string>): string {
  if (!modelId) return ""
  if (nameMap?.[modelId]) return nameMap[modelId]

  // 兼容历史数据：去掉 provider 前缀后匹配旧别名
  const short = modelId.replace(/^(team-deepseek|deepseek|opencode|hy3)\//, "")
  const legacy: Record<string, string> = {
    "deepseek-chat": "DeepSeek V3 Chat",
    "deepseek-v4-flash": "DeepSeek V4 Flash",
    "deepseek-v4-pro": "DeepSeek V4 Pro",
    "deepseek-reasoner": "DeepSeek Reasoner",
    "big-pickle": "Big Pickle (免费)",
    "nemotron-3-super-free": "Nemotron 3 (免费)",
    "hy3-preview": "混元 3 预览",
  }
  return legacy[short] || modelId
}
