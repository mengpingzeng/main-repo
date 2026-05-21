import type { Metadata } from "next"
import { AuthGuard } from "@/components/layout/auth-guard"
import { LayoutSwitcher } from "@/components/layout/layout-switcher"
import "./globals.css"

export const metadata: Metadata = {
  title: "小龙虾创作平台",
  description: "AI 驱动的多平台内容创作与管理平台",
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="zh-CN">
      <body>
        <AuthGuard>
          <LayoutSwitcher>{children}</LayoutSwitcher>
        </AuthGuard>
      </body>
    </html>
  )
}
