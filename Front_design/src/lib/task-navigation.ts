/** 构建任务列表 URL（含分页与搜索） */
export function buildTaskListUrl(page = 1, q = ""): string {
  const params = new URLSearchParams()
  if (page > 1) params.set("page", String(page))
  if (q) params.set("q", q)
  const qs = params.toString()
  return qs ? `/tasks?${qs}` : "/tasks"
}

type TaskDetailNavOpts = {
  sid?: string
  platform?: string
  from: "list" | "new"
  listPage?: number
  listQ?: string
}

/** 进入任务详情时携带来源，供返回按钮还原列表位置 */
export function buildTaskDetailHref(taskId: string, opts: TaskDetailNavOpts): string {
  const params = new URLSearchParams()
  if (opts.sid) params.set("sid", opts.sid)
  if (opts.platform) params.set("platform", opts.platform)
  params.set("from", opts.from)
  if (opts.from === "list") {
    if (opts.listPage && opts.listPage > 1) params.set("listPage", String(opts.listPage))
    if (opts.listQ) params.set("listQ", opts.listQ)
  }
  const qs = params.toString()
  return qs ? `/tasks/${taskId}?${qs}` : `/tasks/${taskId}`
}

/** 根据详情页 query 解析应返回的任务列表 URL */
export function resolveTaskListReturnUrl(searchParams: Pick<URLSearchParams, "get">): string {
  if (searchParams.get("from") === "list") {
    const page = Math.max(1, Number(searchParams.get("listPage")) || 1)
    const q = searchParams.get("listQ") || ""
    return buildTaskListUrl(page, q)
  }
  return buildTaskListUrl(1)
}
