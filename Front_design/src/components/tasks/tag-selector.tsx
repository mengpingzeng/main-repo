"use client"

import { useState, useEffect, useCallback } from "react"
import { createPortal } from "react-dom"
import { cn } from "@/lib/utils"
import { TagChip } from "./tag-chip"
import { TAG_CATEGORIES } from "@/lib/tags"
import { showToast } from "@/components/ui/toast"
import type { TagItem, TagCategoryKey } from "@/lib/tags"
import { ChevronDown, X } from "lucide-react"

export type SelectedTags = Record<TagCategoryKey, TagItem[]>

export function getEmptySelection(): SelectedTags {
  return { main: [], theme: [], role: [], plot: [] }
}

function deepCloneSelection(source: SelectedTags): SelectedTags {
  const clone: SelectedTags = { main: [], theme: [], role: [], plot: [] }
  for (const key of Object.keys(source) as TagCategoryKey[]) {
    clone[key] = [...source[key]]
  }
  return clone
}

function getAllSelectedTags(sel: SelectedTags): (TagItem & { categoryKey: TagCategoryKey })[] {
  const result: (TagItem & { categoryKey: TagCategoryKey })[] = []
  for (const cat of TAG_CATEGORIES) {
    for (const tag of sel[cat.key]) {
      result.push({ ...tag, categoryKey: cat.key })
    }
  }
  return result
}

interface TagInputBarProps {
  selectedTags: SelectedTags
  onRemoveTag: (categoryKey: TagCategoryKey, tagId: string) => void
  onClick: () => void
}

export function TagInputBar({ selectedTags, onRemoveTag, onClick }: TagInputBarProps) {
  const allTags = getAllSelectedTags(selectedTags)

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onClick}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault()
          onClick()
        }
      }}
      className={cn(
        "flex items-center gap-2 w-full min-h-[44px] px-4 py-2.5",
        "bg-background border border-input rounded-lg",
        "cursor-pointer hover:border-primary/50 transition-colors"
      )}
    >
      {allTags.length === 0 ? (
        <span className="text-muted-foreground text-sm flex-1">请选择作品标签</span>
      ) : (
        <div className="flex flex-wrap gap-1.5 flex-1">
          {allTags.map((tag) => (
            <TagChip
              key={`${tag.categoryKey}-${tag.id}`}
              name={tag.name}
              onRemove={() => onRemoveTag(tag.categoryKey, tag.id)}
            />
          ))}
        </div>
      )}
      <ChevronDown className="w-4 h-4 text-muted-foreground shrink-0" />
    </div>
  )
}

interface TagSelectorProps {
  open: boolean
  selectedTags: SelectedTags
  onConfirm: (tags: SelectedTags) => void
  onClose: () => void
}

export function TagSelector({ open, selectedTags, onConfirm, onClose }: TagSelectorProps) {
  const [mounted, setMounted] = useState(false)
  const [localSelection, setLocalSelection] = useState<SelectedTags>(getEmptySelection())
  const [activeTab, setActiveTab] = useState<TagCategoryKey>("main")

  useEffect(() => { setMounted(true) }, [])

  useEffect(() => {
    if (open) {
      setLocalSelection(deepCloneSelection(selectedTags))
      setActiveTab("main")
      document.body.style.overflow = "hidden"
    }
    return () => { document.body.style.overflow = "" }
  }, [open, selectedTags])

  useEffect(() => {
    if (!open) return
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose()
    }
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [open, onClose])

  const activeCategory = TAG_CATEGORIES.find((c) => c.key === activeTab)

  const isTagSelected = useCallback(
    (catKey: TagCategoryKey, tagId: string) =>
      localSelection[catKey].some((t) => t.id === tagId),
    [localSelection]
  )

  const handleToggleTag = useCallback(
    (catKey: TagCategoryKey, tag: TagItem) => {
      setLocalSelection((prev) => {
        const cat = TAG_CATEGORIES.find((c) => c.key === catKey)!
        const current = prev[catKey]
        const isSelected = current.some((t) => t.id === tag.id)
        if (isSelected) return { ...prev, [catKey]: current.filter((t) => t.id !== tag.id) }
        if (current.length >= cat.maxSelect) return prev
        return { ...prev, [catKey]: [...current, tag] }
      })
    },
    []
  )

  const handleRemoveFromSelected = useCallback(
    (catKey: TagCategoryKey, tagId: string) => {
      setLocalSelection((prev) => ({
        ...prev,
        [catKey]: prev[catKey].filter((t) => t.id !== tagId),
      }))
    },
    []
  )

  const handleConfirm = useCallback(() => {
    if (localSelection.main.length === 0) {
      showToast("请选择主分类")
      setActiveTab("main")
      return
    }
    onConfirm(deepCloneSelection(localSelection))
  }, [localSelection, onConfirm])

  if (!mounted || !open) return null

  const allSelectedInModal = getAllSelectedTags(localSelection)

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div className={cn(
        "relative z-10 w-[640px] max-h-[80vh] h-[520px] rounded-xl border border-border bg-background shadow-[0_8px_30px_rgba(0,0,0,0.12)] flex flex-col"
      )}>
        <div className="flex items-center justify-between px-5 py-4 border-b border-border shrink-0">
          <h2 className="text-lg font-bold text-foreground">作品标签</h2>
          <button type="button" onClick={onClose} className="p-1 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="px-5 py-3 border-b border-border shrink-0">
          <div className="flex items-start gap-2">
            <span className="text-sm text-muted-foreground whitespace-nowrap pt-0.5">已选：</span>
            <div className="flex flex-wrap gap-1.5 flex-1 min-h-[24px]">
              {allSelectedInModal.length === 0 ? (
                <span className="text-sm text-muted-foreground">暂无</span>
              ) : (
                allSelectedInModal.map((tag) => (
                  <TagChip key={`${tag.categoryKey}-${tag.id}`} name={tag.name}
                    onRemove={() => handleRemoveFromSelected(tag.categoryKey, tag.id)} />
                ))
              )}
            </div>
          </div>
        </div>

        <div className="flex flex-1 min-h-0 overflow-hidden">
          <div className="w-[88px] shrink-0 border-r border-border px-2 py-3 flex flex-col gap-1">
            {TAG_CATEGORIES.map((cat) => (
              <button key={cat.key} type="button" onClick={() => setActiveTab(cat.key)}
                className={cn(
                  "w-full text-left px-3 py-2.5 rounded-lg text-sm font-medium transition-all",
                  activeTab === cat.key
                    ? "bg-primary/10 text-primary border-l-2 border-primary rounded-l-none"
                    : "text-muted-foreground hover:text-foreground hover:bg-muted"
                )}>
                {cat.label}
              </button>
            ))}
          </div>

          <div className="flex-1 p-4 overflow-y-auto min-h-[340px]">
            {activeCategory && (
              <div className="grid grid-cols-3 gap-2.5">
                {activeCategory.items.map((tag) => {
                  const selected = isTagSelected(activeCategory.key, tag.id)
                  const maxedOut = !selected && localSelection[activeCategory.key].length >= activeCategory.maxSelect
                  return (
                    <button key={tag.id} type="button" disabled={maxedOut}
                      onClick={() => handleToggleTag(activeCategory.key, tag)}
                      className={cn(
                        "px-3 py-2 rounded-lg border text-sm font-medium transition-all text-center",
                        selected && "border-primary bg-primary/10 text-primary",
                        !selected && maxedOut && "border-border bg-muted text-muted-foreground opacity-40 cursor-not-allowed",
                        !selected && !maxedOut && "border-border bg-muted text-foreground hover:border-primary/50"
                      )}>
                      {tag.name}
                    </button>
                  )
                })}
              </div>
            )}
          </div>
        </div>

        <div className="border-t border-border px-5 py-3 shrink-0">
          <p className="text-xs text-muted-foreground mb-3 text-center">
            主分类必选且只能选一个，主题、角色、情节最多可选两个
          </p>
          <div className="flex justify-end gap-3">
            <button type="button" onClick={onClose}
              className="px-4 py-2 text-sm font-medium text-foreground bg-muted border border-border rounded-lg hover:bg-border transition-colors">取消</button>
            <button type="button" onClick={handleConfirm}
              className="px-4 py-2 text-sm font-medium text-white bg-primary rounded-lg hover:bg-[#9e6aff] transition-colors">确认</button>
          </div>
        </div>
      </div>
    </div>,
    document.body
  )
}
