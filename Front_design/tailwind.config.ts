import type { Config } from "tailwindcss"

const config: Config = {
  content: [
    "./src/components/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        background: "#f8fafc",   // slate-50
        foreground: "#0f172a",   // slate-900
        primary: {
          DEFAULT: "#f97316",    // orange-500
          hover:   "#ea6c10",
          active:  "#c2540a",
          light:   "#fff7ed",    // orange-50
          foreground: "#ffffff",
        },
        secondary: {
          DEFAULT: "#f1f5f9",    // slate-100
          foreground: "#0f172a",
        },
        muted: {
          DEFAULT: "#f1f5f9",
          foreground: "#64748b", // slate-500
        },
        destructive: {
          DEFAULT: "#ef4444",    // red-500
          foreground: "#ffffff",
        },
        border:  "#e2e8f0",      // slate-200
        ring:    "#f97316",      // orange-500
        input:   "#e2e8f0",
        accent: {
          DEFAULT:    "#fff7ed", // orange-50
          foreground: "#c2540a", // orange-700
        },
        card: {
          DEFAULT:    "#ffffff",
          foreground: "#0f172a",
        },
        popover: {
          DEFAULT:    "#ffffff",
          foreground: "#0f172a",
        },
        sidebar: "#ffffff",
        stat:    "#fff7ed",
      },
      borderRadius: {
        sm:   "4px",
        md:   "6px",
        lg:   "8px",
        xl:   "12px",
        "2xl":"16px",
      },
      fontFamily: {
        sans: ["var(--font-inter)", '"PingFang SC"', '"Microsoft YaHei"', '"Hiragino Sans GB"', "Arial", "sans-serif"],
      },
      keyframes: {
        "fade-in": {
          from: { opacity: "0", transform: "translateY(-4px)" },
          to:   { opacity: "1", transform: "translateY(0)" },
        },
        "fade-out": {
          from: { opacity: "1", transform: "translateY(0)" },
          to:   { opacity: "0", transform: "translateY(-4px)" },
        },
      },
      animation: {
        "fade-in":  "fade-in 0.2s ease-out",
        "fade-out": "fade-out 0.2s ease-out",
      },
    },
  },
  plugins: [require("tailwindcss-animate")],
}

export default config
