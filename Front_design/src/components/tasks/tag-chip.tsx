"use client"

import { cn } from "@/lib/utils"

interface TagChipProps {
  name: string
  onRemove?: () => void
  className?: string
}

export function TagChip({ name, onRemove, className }: TagChipProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs font-medium border transition-colors",
        "bg-primary/10 text-primary border-primary/20",
        className
      )}
    >
      {name}
      {onRemove && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation()
            onRemove()
          }}
          className="ml-0.5 rounded-full hover:bg-primary/20 hover:text-primary transition-colors leading-none"
          aria-label={`移除 ${name}`}
        >
          ×
        </button>
      )}
    </span>
  )
}
