import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-lg text-sm font-medium transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-500/40 focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        default:     "bg-gradient-to-r from-orange-500 to-red-500 text-white shadow-sm hover:opacity-90 active:opacity-80",
        destructive: "bg-destructive text-destructive-foreground shadow-sm hover:bg-red-600",
        outline:     "border border-slate-200 bg-white text-slate-600 hover:bg-slate-50 hover:text-slate-900",
        secondary:   "bg-slate-100 text-slate-700 hover:bg-slate-200",
        ghost:       "text-slate-600 hover:bg-slate-100 hover:text-slate-900",
        link:        "text-orange-600 underline-offset-4 hover:underline",
      },
      size: {
        sm:   "h-7 rounded-md px-3 text-xs",
        md:   "h-9 px-4 text-sm",
        lg:   "h-10 px-5 text-sm",
        xl:   "h-10 px-6 text-sm w-full",
        icon: "h-9 w-9",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "md",
    },
  }
)

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean
}

export function Button({
  className,
  variant,
  size,
  asChild = false,
  ...props
}: ButtonProps) {
  const Comp = asChild ? Slot : "button"
  return (
    <Comp
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}
