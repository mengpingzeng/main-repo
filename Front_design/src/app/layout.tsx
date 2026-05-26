import type { Metadata } from "next"
import { Inter } from "next/font/google"
import { AuthGuard } from "@/components/layout/auth-guard"
import { LayoutSwitcher } from "@/components/layout/layout-switcher"
import { ToastContainer } from "@/components/ui/toast"
import "./globals.css"

const inter = Inter({
  subsets: ["latin"],
  weight: ["300", "400", "500", "600", "700"],
  display: "swap",
  variable: "--font-inter",
})

export const metadata: Metadata = {
  title: "铸文坊",
  description: "智叙千篇，匠造万象",
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN" className={inter.variable}>
      <body className={inter.className}>
        <AuthGuard>
          <LayoutSwitcher>{children}</LayoutSwitcher>
        </AuthGuard>
        <ToastContainer />
      </body>
    </html>
  )
}
