"use client"

import * as React from "react"
import * as SelectPrimitive from "@radix-ui/react-select"
import { ChevronDown } from "lucide-react"
import { cn } from "@/lib/utils"

interface SelectProps {
  value: string
  onValueChange: (value: string) => void
  placeholder?: string
  className?: string
  disabled?: boolean
  children: React.ReactNode
}

export function Select({ value, onValueChange, placeholder, className, disabled, children }: SelectProps) {
  return (
    <SelectPrimitive.Root value={value} onValueChange={onValueChange} disabled={disabled}>
      <SelectPrimitive.Trigger
        className={cn(
          "flex w-full items-center justify-between rounded-lg border border-input bg-background px-3 py-2.5 text-sm",
          "transition-colors",
          "placeholder:text-slate-400",
          "focus-visible:outline-none focus-visible:border-orange-400 focus-visible:ring-2 focus-visible:ring-orange-500/15",
          "disabled:cursor-not-allowed disabled:opacity-50",
          "cursor-pointer group",
          className
        )}
      >
        <SelectPrimitive.Value placeholder={placeholder ?? "请选择"} />
        <SelectPrimitive.Icon asChild>
          <ChevronDown className="w-4 h-4 text-muted-foreground transition-transform group-data-[state=open]:rotate-180" />
        </SelectPrimitive.Icon>
      </SelectPrimitive.Trigger>
      <SelectPrimitive.Portal>
        <SelectPrimitive.Content
          position="popper"
          side="bottom"
          sideOffset={4}
          className={cn(
            "z-50 w-[var(--radix-select-trigger-width)] rounded-lg border border-[#e5e6eb] bg-white py-1 shadow-md",
            "max-h-[340px] overflow-y-auto"
          )}
        >
          <SelectPrimitive.Viewport>
            {children}
          </SelectPrimitive.Viewport>
        </SelectPrimitive.Content>
      </SelectPrimitive.Portal>
    </SelectPrimitive.Root>
  )
}

interface SelectItemProps {
  value: string
  children: React.ReactNode
}

export function SelectItem({ value, children }: SelectItemProps) {
  return (
    <SelectPrimitive.Item
      value={value}
      className={cn(
        "relative flex items-center px-3 py-2 text-sm cursor-pointer outline-none",
        "hover:bg-slate-50 focus:bg-slate-50",
        "data-[state=checked]:bg-orange-50 data-[state=checked]:font-medium data-[state=checked]:text-orange-700"
      )}
    >
      <SelectPrimitive.ItemText>{children}</SelectPrimitive.ItemText>
    </SelectPrimitive.Item>
  )
}
