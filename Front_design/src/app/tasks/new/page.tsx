"use client"

import { useEffect, useState, useCallback } from "react"
import { useRouter } from "next/navigation"
import { Textarea, Input } from "@/components/ui/input"
import { Select as SelectRadix, SelectItem } from "@/components/ui/select"
import { fetchModels, fetchSkills, fetchAccounts, createTask, createSession, generateNovelTitle } from "@/lib/api"
import type { Model, Skill, AccountSummary } from "@/types"
import { Loader2 } from "lucide-react"
import { toast } from "@/components/ui/toast"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog"
import { TAG_CATEGORIES, buildPrompt } from "@/lib/tags"
import type { TagItem, TagCategoryKey } from "@/lib/tags"
import { cn } from "@/lib/utils"
import { buildTaskDetailHref } from "@/lib/task-navigation"

type SelectedTags = Record<TagCategoryKey, TagItem[]>
function getEmptySelection(): SelectedTags { return { main: [], theme: [], role: [], plot: [] } }

const PLATFORM_OPTS = [
  { value: "fanqie",  label: "番茄小说", bg: "bg-red-50",  text: "text-red-500",  char: "番" },
  { value: "zhulang", label: "逐浪网",   bg: "bg-blue-50", text: "text-blue-500", char: "逐" },
]

/** 分区标题 */
function SectionHeader({ num, title }: { num: number; title: string }) {
  return (
    <h2 className="text-xl font-semibold mb-6 flex items-center gap-3">
      <span className="w-8 h-8 rounded-full bg-slate-100 text-slate-600 flex items-center justify-center text-sm font-semibold flex-shrink-0">
        {num}
      </span>
      {title}
    </h2>
  )
}

/** 字段标签行 */
function FieldHeader({ label, required, hint }: { label: string; required?: boolean; hint?: string }) {
  return (
    <div className="flex items-center mb-3 gap-2">
      <label className="text-sm font-medium text-slate-700">
        {label} {required && <span className="text-orange-500">*</span>}
      </label>
      {hint && <span className="text-xs text-slate-400">· {hint}</span>}
    </div>
  )
}

export default function NewTaskPage() {
  const router = useRouter()
  const [models, setModels] = useState<Model[]>([])
  const [skills, setSkills] = useState<Skill[]>([])
  const [accounts, setAccounts] = useState<AccountSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)

  const [topic, setTopic] = useState("")
  const [novelName, setNovelName] = useState("")
  const [aiAutoName, setAiAutoName] = useState(false)
  const [aiNaming, setAiNaming] = useState(false)
  const [titleSuggestions, setTitleSuggestions] = useState<{ name: string; desc: string }[]>([])
  const [selectedSuggestion, setSelectedSuggestion] = useState(0)
  const [platform, setPlatform] = useState("fanqie")
  const [skillId, setSkillId] = useState("")
  const [modelId, setModelId] = useState("")
  const [selectedAccountId, setSelectedAccountId] = useState("")
  const [accountLoading, setAccountLoading] = useState(false)
  const [selectedTags, setSelectedTags] = useState<SelectedTags>(getEmptySelection())
  const [collapsedCats, setCollapsedCats] = useState<Record<string, boolean>>({ main: true, theme: true, role: true, plot: true })

  const loadResources = useCallback(async () => {
    setLoading(true)
    try {
      const [m, s] = await Promise.all([fetchModels(), fetchSkills()])
      setModels(m)
      setSkills(s)
      if (s.length > 0) setSkillId(s[0].skill_id)
      if (m.length > 0) {
        const flashModel = m.find(model => model.id === "deepseek/deepseek-v4-flash")
        setModelId(flashModel ? flashModel.id : m[0].id)
      }
    } catch {
      toast.error("加载资源失败，请确认后端服务已启动")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { loadResources() }, [loadResources])

  useEffect(() => {
    setSelectedAccountId("")
    setAccounts([])
    setAccountLoading(true)
    fetchAccounts(platform)
      .then((accs) => setAccounts(Array.isArray(accs) ? accs : []))
      .catch(() => setAccounts([]))
      .finally(() => setAccountLoading(false))
  }, [platform])

  const handleAiName = async () => {
    if (selectedTags.main.length === 0) return
    setAiNaming(true)
    setAiAutoName(true)
    setNovelName("")
    setTitleSuggestions([])
    try {
      const topicStr = buildPrompt(selectedTags, "")
      const { titles } = await generateNovelTitle(topicStr)
      // 解析每行：《书名》—— 描述
      const parsed = titles.map(line => {
        const nameMatch = line.match(/《([^》]+)》/)
        const name = nameMatch ? nameMatch[1] : line.replace(/——.*$/, "").trim()
        const desc = line.replace(/^《[^》]+》\s*——?\s*/, "").trim()
        return { name, desc }
      }).filter(t => t.name)
      if (parsed.length === 0) throw new Error("未解析到书名")
      setTitleSuggestions(parsed)
      setSelectedSuggestion(0)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "AI 命名失败，请重试")
      setAiAutoName(false)
    } finally {
      setAiNaming(false)
    }
  }

  const confirmSuggestion = () => {
    const t = titleSuggestions[selectedSuggestion]
    if (t) setNovelName(t.name)
    setTitleSuggestions([])
    setAiAutoName(false)
  }

  const handleToggleTag = useCallback((catKey: TagCategoryKey, tag: TagItem) => {
    setSelectedTags(prev => {
      const cat = TAG_CATEGORIES.find(c => c.key === catKey)!
      const current = prev[catKey]
      const isSelected = current.some(t => t.id === tag.id)
      if (isSelected) return { ...prev, [catKey]: current.filter(t => t.id !== tag.id) }
      if (current.length >= cat.maxSelect) {
        // 单选直接替换；多选达上限时不操作
        if (cat.maxSelect === 1) return { ...prev, [catKey]: [tag] }
        return prev
      }
      return { ...prev, [catKey]: [...current, tag] }
    })
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (selectedTags.main.length === 0) { toast.error("请选择主分类"); return }
    if (!skillId) { toast.error("请选择写作技能"); return }
    if (!selectedAccountId) { toast.error("请选择发布账号"); return }
    setSubmitting(true)
    try {
      const prompt = buildPrompt(selectedTags, topic)
      const taskResp = await createTask({ topic: prompt, platform, skill_id: skillId, model: modelId, account_ids: [selectedAccountId] })
      const taskId = taskResp.data?.task_id
      if (!taskId) throw new Error("创建任务失败：未返回 task_id")
      let session
      try {
        session = await createSession({ task_id: taskId, skillId, model: modelId, topic: prompt, platform, accountId: selectedAccountId, novel_name: novelName.trim() || undefined })
      } catch (err: unknown) {
        // 若任务已有活跃 session（409），直接使用已有 session 导航
        if (err instanceof Error && err.message) {
          const match = err.message.match(/existing_session_id[": ]+([a-z0-9]+)/)
          if (match?.[1]) {
            router.replace(buildTaskDetailHref(taskId, { sid: match[1], platform, from: "new" }))
            return
          }
        }
        throw err
      }
      router.replace(buildTaskDetailHref(taskId, { sid: session.session_id, platform, from: "new" }))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "创建任务失败")
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <Loader2 className="w-6 h-6 animate-spin text-orange-500" />
      </div>
    )
  }

  return (
    <div className="max-w-5xl mx-auto pb-28">
      {/* 页头 */}
      <header className="mb-10">
        <h1 className="text-3xl font-bold text-slate-900 tracking-tight">新建创作任务</h1>
        <p className="text-slate-500 mt-2 text-sm">选择标签与发布渠道，AI 将为你量身定制创作内容。</p>
      </header>

      <form onSubmit={handleSubmit} className="space-y-8">

        {/* ── Section 1：内容核心设定 ── */}
        <section className="bg-white p-8 rounded-2xl shadow-sm border border-slate-200">
          <SectionHeader num={1} title="内容核心设定" />
          <div className="space-y-8">

            {/* 主分类 */}
            <div>
              <FieldHeader label="主分类" required hint="单选" />
              {(() => {
                const cat = TAG_CATEGORIES.find(c => c.key === "main")!
                const COLS = 6, SHOW = COLS * 2
                const collapsed = collapsedCats["main"]
                const visibleItems = collapsed ? cat.items.slice(0, SHOW) : cat.items
                const canExpand = cat.items.length > SHOW
                return (
                  <>
                    <div className="grid grid-cols-3 sm:grid-cols-6 gap-2.5">
                      {visibleItems.map(tag => {
                        const sel = selectedTags.main.some(t => t.id === tag.id)
                        return (
                          <button
                            key={tag.id} type="button"
                            onClick={() => handleToggleTag("main", tag)}
                            className={cn(
                              "relative px-3 py-2.5 rounded-xl border text-sm font-medium text-center transition-all",
                              sel
                                ? "border-orange-400 bg-orange-50 text-orange-700"
                                : "border-slate-200 bg-white text-slate-600 hover:border-orange-300"
                            )}
                          >
                            {tag.name}
                            {sel && (
                              <span className="absolute -top-1.5 -right-1.5 w-4 h-4 bg-orange-500 rounded-full flex items-center justify-center shadow-sm">
                                <svg className="w-2.5 h-2.5" fill="none" stroke="white" viewBox="0 0 24 24">
                                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={3} d="M5 13l4 4L19 7"/>
                                </svg>
                              </span>
                            )}
                          </button>
                        )
                      })}
                    </div>
                    {canExpand && (
                      <div className="flex justify-end mt-2.5">
                        <button
                          type="button"
                          onClick={() => setCollapsedCats(prev => ({ ...prev, main: !prev.main }))}
                          className="text-xs text-slate-400 hover:text-orange-500 flex items-center gap-1 transition-colors"
                        >
                          {collapsed ? (
                            <>展开全部 {cat.items.length} 项
                              <svg className="w-3 h-3" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth={2}><path d="M2 4l4 4 4-4"/></svg>
                            </>
                          ) : (
                            <>收起
                              <svg className="w-3 h-3" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth={2}><path d="M2 8l4-4 4 4"/></svg>
                            </>
                          )}
                        </button>
                      </div>
                    )}
                  </>
                )
              })()}
            </div>

            {/* 主题 / 角色 / 情节 */}
            {([
              { key: "theme" as TagCategoryKey, label: "主题风格", hint: "最多选 2 项" },
              { key: "role"  as TagCategoryKey, label: "角色设定", hint: "最多选 2 项" },
              { key: "plot"  as TagCategoryKey, label: "情节走向", hint: "最多选 2 项" },
            ]).map(({ key, label, hint }) => {
              const cat = TAG_CATEGORIES.find(c => c.key === key)!
              const COLS = 6
              const SHOW = COLS * 2
              const collapsed = collapsedCats[key]
              const visibleItems = collapsed ? cat.items.slice(0, SHOW) : cat.items
              const canExpand = cat.items.length > SHOW
              return (
                <div key={key}>
                  <FieldHeader label={label} hint={hint} />
                  <div className="grid grid-cols-3 sm:grid-cols-6 gap-2.5">
                    {visibleItems.map(tag => {
                      const sel = selectedTags[key].some(t => t.id === tag.id)
                      const maxed = !sel && selectedTags[key].length >= cat.maxSelect
                      return (
                        <button
                          key={tag.id} type="button"
                          disabled={maxed}
                          onClick={() => handleToggleTag(key, tag)}
                          className={cn(
                            "px-3 py-2.5 rounded-xl border text-sm text-center transition-all",
                            sel && "border-orange-400 bg-orange-50 text-orange-700 font-medium",
                            !sel && !maxed && "border-slate-200 bg-white text-slate-600 hover:border-orange-300",
                            maxed && "border-slate-100 bg-slate-50 text-slate-300 cursor-not-allowed"
                          )}
                        >
                          {tag.name}
                        </button>
                      )
                    })}
                  </div>
                  {canExpand && (
                    <div className="flex justify-end mt-2.5">
                      <button
                        type="button"
                        onClick={() => setCollapsedCats(prev => ({ ...prev, [key]: !prev[key] }))}
                        className="text-xs text-slate-400 hover:text-orange-500 flex items-center gap-1 transition-colors"
                      >
                        {collapsed ? (
                          <>展开全部 {cat.items.length} 项
                            <svg className="w-3 h-3" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth={2}><path d="M2 4l4 4 4-4"/></svg>
                          </>
                        ) : (
                          <>收起
                            <svg className="w-3 h-3" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth={2}><path d="M2 8l4-4 4 4"/></svg>
                          </>
                        )}
                      </button>
                    </div>
                  )}
                </div>
              )
            })}

            {/* 小说名称 */}
            <div>
              <div className="flex items-center mb-3">
                <label className="text-sm font-medium text-slate-700">
                  小说名称 <span className="text-orange-500">*</span>
                </label>
              </div>
              <div className="flex gap-2">
                <Input
                  value={novelName}
                  onChange={e => setNovelName(e.target.value)}
                  disabled={aiNaming}
                  placeholder={aiAutoName && !novelName ? "AI 正在生成书名..." : "请输入小说名称..."}
                  className={cn("h-11 px-4 text-sm flex-1", aiNaming && "bg-slate-50 text-slate-400")}
                />
                <button
                  type="button"
                  disabled={selectedTags.main.length === 0 || aiNaming}
                  onClick={handleAiName}
                  title={selectedTags.main.length === 0 ? "请先选择主分类" : ""}
                  className={cn(
                    "h-11 px-4 text-sm font-medium rounded-lg whitespace-nowrap transition-all flex-shrink-0 flex items-center gap-1.5",
                    selectedTags.main.length === 0 || aiNaming
                      ? "bg-slate-100 text-slate-400 cursor-not-allowed"
                      : "bg-gradient-to-r from-orange-500 to-red-500 text-white hover:opacity-90 shadow-sm"
                  )}
                >
                  {aiNaming ? <><Loader2 size={13} className="animate-spin" />生成中...</> : "自动生成"}
                </button>
              </div>
              {selectedTags.main.length === 0 && (
                <p className="mt-1.5 text-xs text-slate-400"></p>
              )}
            </div>

            {/* 作品描述 */}
            <div>
              <FieldHeader label="作品描述" hint="选填" />
              <Textarea
                value={topic}
                onChange={e => setTopic(e.target.value)}
                placeholder="可输入补充描述，帮助 AI 更精准地创作..."
                rows={3}
                maxLength={500}
              />
              <p className="text-xs text-slate-400 mt-1.5 text-right">{topic.length}/500</p>
            </div>
          </div>
        </section>

        {/* ── Section 2：分发与引擎设定 ── */}
        <section className="bg-white p-8 rounded-2xl shadow-sm border border-slate-200">
          <SectionHeader num={2} title="分发与引擎设定" />
          <div className="space-y-8">

            {/* 目标发布平台 */}
            <div>
              <FieldHeader label="目标发布平台" required hint="单选" />
              <div className="grid grid-cols-5 gap-3">
                {PLATFORM_OPTS.map(opt => {
                  const sel = platform === opt.value
                  return (
                    <button
                      key={opt.value} type="button"
                      onClick={() => setPlatform(opt.value)}
                      className={cn(
                        "relative flex flex-col items-center justify-center gap-2 py-5 rounded-xl border-2 transition-all",
                        sel ? "border-orange-400 bg-orange-50" : "border-slate-200 bg-white hover:border-orange-300"
                      )}
                    >
                      {sel && (
                        <span className="absolute -top-2 -right-2 w-5 h-5 bg-orange-500 rounded-full flex items-center justify-center shadow-sm">
                          <svg className="w-3 h-3" fill="none" stroke="white" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={3} d="M5 13l4 4L19 7"/>
                          </svg>
                        </span>
                      )}
                      <div className={cn("w-10 h-10 rounded-full flex items-center justify-center font-bold text-base", opt.bg, opt.text)}>
                        {opt.char}
                      </div>
                      <span className={cn("text-sm font-medium", sel ? "text-orange-700" : "text-slate-700")}>
                        {opt.label}
                      </span>
                    </button>
                  )
                })}
              </div>
            </div>

            {/* 发布账号 */}
            <div>
              <FieldHeader label="发布账号" required hint="单选" />
              <div className="min-h-9">
                {accountLoading ? (
                  <div className="flex flex-wrap gap-2.5">
                    <div
                      aria-hidden
                      className="h-9 w-24 rounded-lg border border-slate-200 bg-slate-100 animate-pulse"
                    />
                    <div
                      aria-hidden
                      className="h-9 w-28 rounded-lg border border-slate-200 bg-slate-100 animate-pulse"
                    />
                    <div className="inline-flex h-9 items-center gap-2 px-4 rounded-lg border border-slate-200 bg-slate-50 text-sm text-slate-400">
                      <Loader2 size={14} className="animate-spin shrink-0" />
                      <span>加载中...</span>
                    </div>
                  </div>
                ) : accounts.length === 0 ? (
                  <p className="flex h-9 items-center text-sm text-slate-400">
                    该平台暂无绑定账号，请先到
                    <a href="/accounts" className="text-orange-500 underline mx-1">账号配置</a>
                    绑定
                  </p>
                ) : (
                  <div className="flex flex-wrap gap-2.5">
                    {accounts.map(acc => {
                      const sel = selectedAccountId === acc.account_id
                      return (
                        <button
                          key={acc.account_id} type="button"
                          onClick={() => setSelectedAccountId(acc.account_id)}
                          className={cn(
                            "inline-flex h-9 items-center gap-2 px-4 rounded-lg border text-sm transition-all",
                            sel
                              ? "border-orange-400 bg-orange-50 text-orange-700 font-medium"
                              : "border-slate-200 bg-white text-slate-600 hover:border-orange-300"
                          )}
                        >
                          {sel && (
                            <span className="w-3.5 h-3.5 rounded-full bg-orange-500 flex items-center justify-center flex-shrink-0">
                              <svg className="w-2" fill="none" stroke="white" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={3} d="M5 13l4 4L19 7"/>
                              </svg>
                            </span>
                          )}
                          {acc.masked_display}
                        </button>
                      )
                    })}
                  </div>
                )}
              </div>
            </div>

            {/* AI 引擎 + 写作技能 */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6 pt-6 border-t border-slate-100">
              <div>
                <label className="block text-sm font-medium text-slate-700 mb-2">底层 AI 引擎 <span className="text-orange-500">*</span></label>
                <SelectRadix value={modelId} onValueChange={setModelId} className="h-12 px-4 text-sm">
                  {models.map(m => (
                    <SelectItem key={m.id} value={m.id}>{m.name}</SelectItem>
                  ))}
                </SelectRadix>
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-700 mb-2">
                  预设写作技能 (Skill) <span className="text-orange-500">*</span>
                </label>
                <SelectRadix value={skillId} onValueChange={setSkillId} className="h-12 px-4 text-sm">
                  {skills.map(s => (
                    <SelectItem key={s.skill_id} value={s.skill_id}>{s.name}</SelectItem>
                  ))}
                </SelectRadix>
              </div>
            </div>
          </div>
        </section>

      </form>

      {/* AI 候选书名 Dialog */}
      <Dialog open={titleSuggestions.length > 0} onOpenChange={open => { if (!open) { setTitleSuggestions([]); setAiAutoName(false) } }}>
        <DialogContent className="max-w-md" onPointerDownOutside={e => e.preventDefault()}>
          <DialogHeader>
            <DialogTitle className="text-center">选择小说名称</DialogTitle>
          </DialogHeader>
          <div className="space-y-2 py-2">
            {titleSuggestions.map((t, i) => (
              <button
                key={i} type="button"
                onClick={() => setSelectedSuggestion(i)}
                className={cn(
                  "w-full flex items-start gap-3 px-4 py-3 rounded-xl border text-left transition-all",
                  selectedSuggestion === i
                    ? "border-orange-400 bg-orange-50"
                    : "border-slate-200 bg-white hover:border-orange-300"
                )}
              >
                <span className={cn(
                  "mt-0.5 w-4 h-4 rounded-full border-2 flex-shrink-0 flex items-center justify-center",
                  selectedSuggestion === i ? "border-orange-500" : "border-slate-300"
                )}>
                  {selectedSuggestion === i && <span className="w-2 h-2 rounded-full bg-orange-500" />}
                </span>
                <span className="flex-1 min-w-0">
                  <span className="block text-sm font-semibold text-slate-900">《{t.name}》</span>
                  <span className="block text-xs text-slate-500 mt-1 leading-relaxed">{t.desc}</span>
                </span>
              </button>
            ))}
          </div>
          <DialogFooter>
            <button
              type="button"
              onClick={() => { setTitleSuggestions([]); setAiAutoName(false) }}
              className="px-4 py-2 text-sm text-slate-500 hover:text-slate-700 transition-colors"
            >
              取消
            </button>
            <button
              type="button"
              onClick={confirmSuggestion}
              className="px-6 py-2 text-sm font-medium text-white bg-gradient-to-r from-orange-500 to-red-500 rounded-lg hover:opacity-90 shadow-sm transition-all"
            >
              确认
            </button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ── 固定底部栏 ── */}
      <div className="fixed bottom-0 left-64 right-0 bg-white border-t border-slate-200 px-8 py-4 z-10">
        <div className="max-w-5xl mx-auto flex justify-end">
          <button
            type="submit"
            form="new-task-form"
            disabled={submitting}
            onClick={handleSubmit}
            className="px-7 py-2.5 text-sm font-medium text-white bg-gradient-to-r from-orange-500 to-red-500 rounded-lg hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed shadow-sm flex items-center gap-2 transition-all"
          >
            {submitting
              ? <><Loader2 size={14} className="animate-spin" />创建中...</>
              : "创建任务并进入工作台"
            }
          </button>
        </div>
      </div>
    </div>
  )
}
