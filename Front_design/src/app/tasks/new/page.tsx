"use client"

import { useEffect, useState, useCallback } from "react"
import { useRouter } from "next/navigation"
import { Button } from "@/components/ui/button"
import { Label, Textarea } from "@/components/ui/input"
import { Select as SelectRadix, SelectItem } from "@/components/ui/select"
import { fetchModels, fetchSkills, fetchAccounts, createTask, createSession } from "@/lib/api"
import type { Model, Skill, AccountSummary } from "@/types"
import { Loader2 } from "lucide-react"
import {
  TagInputBar,
  TagSelector,
  getEmptySelection,
} from "@/components/tasks/tag-selector"
import { ToastContainer } from "@/components/ui/toast"
import { buildPrompt } from "@/lib/tags"
import type { TagCategoryKey } from "@/lib/tags"
import { cn } from "@/lib/utils"

function FormRow({
  label,
  children,
  required,
  noBorder,
}: {
  label: string
  children: React.ReactNode
  required?: boolean
  noBorder?: boolean
}) {
  return (
    <div
      className={cn(
        "flex flex-col md:flex-row md:items-start gap-2 md:gap-6 py-3 first:pt-0 last:pb-0",
        !noBorder && "border-b border-[#f0f0f0] last:border-0"
      )}
    >
      <Label className="md:w-[80px] md:min-w-[80px] md:pt-2 md:mb-0 shrink-0">
        {label}
        {required && <span className="text-destructive ml-0.5">*</span>}
      </Label>
      <div className="flex-1 min-w-0">{children}</div>
    </div>
  )
}

function SectionTitle({ children }: { children: React.ReactNode }) {
  return (
    <h2 className="px-5 py-2.5 bg-[#f7f8fa] border-b border-[#f0f0f0] text-sm font-semibold text-foreground rounded-t-lg">
      {children}
    </h2>
  )
}

export default function NewTaskPage() {
  const router = useRouter()
  const [models, setModels] = useState<Model[]>([])
  const [skills, setSkills] = useState<Skill[]>([])
  const [accounts, setAccounts] = useState<AccountSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState("")

  const [topic, setTopic] = useState("")
  const [platform, setPlatform] = useState("fanqie")
  const [skillId, setSkillId] = useState("")
  const [modelId, setModelId] = useState("deepseek/deepseek-chat")
  const [selectedAccountId, setSelectedAccountId] = useState("")
  const [accountLoading, setAccountLoading] = useState(false)
  const [selectedTags, setSelectedTags] = useState(getEmptySelection())
  const [tagSelectorOpen, setTagSelectorOpen] = useState(false)

  const loadResources = useCallback(async () => {
    setLoading(true)
    try {
      const [m, s] = await Promise.all([
        fetchModels(),
        fetchSkills(),
      ])
      setModels(m)
      setSkills(s)
      if (s.length > 0) setSkillId(s[0].skill_id)
    } catch {
      setError("加载资源失败，请确认后端服务已启动")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadResources()
  }, [loadResources])

  useEffect(() => {
    setSelectedAccountId("")
    setAccountLoading(true)
    fetchAccounts(platform)
      .then((accs) => setAccounts(Array.isArray(accs) ? accs : []))
      .catch(() => setAccounts([]))
      .finally(() => setAccountLoading(false))
  }, [platform])

  const handleRemoveTag = (categoryKey: TagCategoryKey, tagId: string) => {
    setSelectedTags((prev) => ({
      ...prev,
      [categoryKey]: prev[categoryKey].filter((t) => t.id !== tagId),
    }))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (selectedTags.main.length === 0) {
      setError("请选择作品标签的主分类")
      return
    }
    if (!skillId) {
      setError("请选择写作风格")
      return
    }
    if (!selectedAccountId) {
      setError("请选择发布账号")
      return
    }

    setSubmitting(true)
    setError("")

    try {
      const prompt = buildPrompt(selectedTags, topic)
      const taskResp = await createTask({
        topic: prompt,
        platform,
        skill_id: skillId,
        model: modelId,
        account_ids: [selectedAccountId],
      })
      const taskId = taskResp.data?.task_id
      if (!taskId) throw new Error("创建任务失败：未返回 task_id")
      const session = await createSession({
        task_id: taskId,
        skillId,
        model: modelId,
        topic: prompt,
        platform,
        accountId: selectedAccountId,
      })
      router.push(`/tasks/${taskId}?sid=${session.session_id}&platform=${platform}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : "创建任务失败")
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <Loader2 className="w-6 h-6 animate-spin text-primary" />
      </div>
    )
  }

  return (
    <div className="w-full max-w-[1080px]">
      <div className="mb-5">
        <h1 className="text-lg font-semibold text-foreground">
          新建创作任务
        </h1>
        <p className="text-[13px] text-muted-foreground mt-1.5">
          填写作品标签与补充说明，选择发布平台与写作风格后提交。
        </p>
      </div>

      <form onSubmit={handleSubmit} className="space-y-4">
        {/* 基本设置 */}
        <div className="bg-white rounded-lg border border-[#e5e6eb]">
          <SectionTitle>基本设置</SectionTitle>
          <div className="px-5 py-4">
            <FormRow label="作品标签" required>
              <TagInputBar
                selectedTags={selectedTags}
                onRemoveTag={handleRemoveTag}
                onClick={() => setTagSelectorOpen(true)}
              />
            </FormRow>

            <FormRow label="作品描述">
              <div>
                <Textarea
                  value={topic}
                  onChange={(e) => setTopic(e.target.value)}
                  placeholder="请输入补充描述（选填）"
                  rows={3}
                  maxLength={500}
                />
                <p className="text-xs text-muted-foreground mt-1 text-right">
                  {topic.length}/500
                </p>
              </div>
            </FormRow>
          </div>

          <div className="border-t border-[#f0f0f0] px-5 py-4">
            <FormRow label="发布平台" noBorder>
              <SelectRadix value={platform} onValueChange={setPlatform}>
                <SelectItem value="xhs">小红书</SelectItem>
                <SelectItem value="wechat">微信公众号</SelectItem>
                <SelectItem value="fanqie">番茄小说</SelectItem>
                <SelectItem value="yuewen">阅文</SelectItem>
                <SelectItem value="zhulang">逐浪网</SelectItem>
              </SelectRadix>
            </FormRow>
            <FormRow label="AI 模型" noBorder>
              <SelectRadix value={modelId} onValueChange={setModelId}>
                {models.map((m) => (
                  <SelectItem key={m.id} value={m.id}>
                    {m.name}
                  </SelectItem>
                ))}
              </SelectRadix>
            </FormRow>
          </div>
        </div>

        {/* 写作风格 */}
        <div className="bg-white rounded-lg border border-[#e5e6eb]">
          <SectionTitle>写作风格</SectionTitle>
          <div className="px-5 py-4">
            <div className="rounded-md border border-[#e5e6eb] overflow-hidden divide-y divide-[#f0f0f0] max-h-[320px] overflow-y-auto">
              {skills.map((s) => {
                const selected = skillId === s.skill_id
                return (
                  <button
                    key={s.skill_id}
                    type="button"
                    onClick={() => setSkillId(s.skill_id)}
                    className={cn(
                      "w-full flex items-start gap-3 px-4 py-3 text-left transition-colors",
                      selected
                        ? "bg-[#f7f8fa]"
                        : "bg-white hover:bg-[#fafafa]"
                    )}
                    style={selected ? { boxShadow: "inset 2px 0 0 0 #635bff" } : undefined}
                  >
                    <span
                      className={cn(
                        "mt-0.5 w-4 h-4 rounded-full border shrink-0 flex items-center justify-center",
                        selected ? "border-[#635bff]" : "border-[#c9cdd4]"
                      )}
                    >
                      {selected && <span className="w-2 h-2 rounded-full bg-[#635bff]" />}
                    </span>
                    <span className="flex-1 min-w-0">
                      <span className={cn("text-sm block", selected ? "font-medium text-[#1d2129]" : "text-[#4e5969]")}>
                        {s.name}
                      </span>
                      <span className="text-xs text-[#86909c] mt-0.5 line-clamp-2 block">
                        {s.description}
                      </span>
                    </span>
                  </button>
                )
              })}
            </div>
          </div>
        </div>

        {/* 发布账号 */}
        <div className="bg-white rounded-lg border border-[#e5e6eb]">
          <SectionTitle>发布账号<span className="text-destructive ml-0.5">*</span></SectionTitle>
          <div className="px-5 py-4">
            {accountLoading ? (
              <p className="text-xs text-muted-foreground">加载中...</p>
            ) : accounts.length === 0 ? (
              <p className="text-xs text-muted-foreground">
                该平台暂无绑定账号，请先到
                <a href="/accounts" className="text-primary underline mx-1">账号管理</a>
                绑定
              </p>
            ) : (
              <div className="rounded-md border border-[#e5e6eb] overflow-hidden divide-y divide-[#f0f0f0]">
                {accounts.map((acc) => {
                  const selected = selectedAccountId === acc.account_id
                  return (
                    <button
                      key={acc.account_id}
                      type="button"
                      onClick={() => setSelectedAccountId(acc.account_id)}
                      className={cn(
                        "w-full flex items-center gap-3 px-4 py-2.5 text-left transition-colors",
                        selected
                          ? "bg-[#f7f8fa]"
                          : "bg-white hover:bg-[#fafafa]"
                      )}
                      style={selected ? { boxShadow: "inset 2px 0 0 0 #635bff" } : undefined}
                    >
                      <span
                        className={cn(
                          "w-4 h-4 rounded-full border shrink-0 flex items-center justify-center",
                          selected ? "border-[#635bff]" : "border-[#c9cdd4]"
                        )}
                      >
                        {selected && <span className="w-2 h-2 rounded-full bg-[#635bff]" />}
                      </span>
                      <span className="text-sm text-[#1d2129]">{acc.masked_display}</span>
                      <span className="text-[11px] text-muted-foreground font-mono">{acc.account_id}</span>
                    </button>
                  )
                })}
              </div>
            )}
          </div>
        </div>

        {/* 错误 */}
        {error && (
          <div className="p-3 rounded-lg bg-destructive/8 border border-destructive/20 text-destructive text-sm">
            {error}
          </div>
        )}

        {/* 提交按钮 */}
        <div className="flex justify-end pt-1">
          <Button
            type="submit"
            size="lg"
            disabled={submitting}
            className="min-w-[120px]"
          >
            {submitting ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                正在创建...
              </>
            ) : (
              "开始创作"
            )}
          </Button>
        </div>
      </form>

      <TagSelector
        open={tagSelectorOpen}
        selectedTags={selectedTags}
        onConfirm={(tags) => {
          setSelectedTags(tags)
          setTagSelectorOpen(false)
        }}
        onClose={() => setTagSelectorOpen(false)}
      />

      <ToastContainer />
    </div>
  )
}
