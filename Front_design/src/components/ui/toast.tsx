"use client"

import { Toaster as Sonner } from "sonner"

export { toast } from "sonner"

export function showToast(msg: string) {
  const { toast } = require("sonner")
  toast.error(msg)
}

export function ToastContainer() {
  return (
    <Sonner
      position="top-center"
      toastOptions={{
        style: {
          background: "#ffffff",
          border: "1px solid #e5e6eb",
          color: "#1d2129",
          borderRadius: "8px",
          fontSize: "14px",
          boxShadow: "0 4px 16px rgba(0,0,0,0.08)",
        },
      }}
    />
  )
}
