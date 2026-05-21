import type { Config } from "tailwindcss"

const config: Config = {
  content: [
    "./src/components/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        background: "#ffffff",
        foreground: "#1d2129",
        primary: {
          DEFAULT: "#635bff",
          hover: "#5548e6",
          active: "#4a3fd4",
          light: "#f0eeff",
          foreground: "#ffffff",
        },
        secondary: {
          DEFAULT: "#f7f8fa",
          foreground: "#1d2129",
        },
        muted: {
          DEFAULT: "#f2f3f5",
          foreground: "#86909c",
        },
        destructive: {
          DEFAULT: "#ff4d4f",
          foreground: "#ffffff",
        },
        border: "#e5e6eb",
        ring: "#635bff",
        input: "#e5e6eb",
        accent: {
          DEFAULT: "#f0eeff",
          foreground: "#635bff",
        },
        card: {
          DEFAULT: "#ffffff",
          foreground: "#1d2129",
        },
        popover: {
          DEFAULT: "#ffffff",
          foreground: "#1d2129",
        },
        sidebar: "#f7f8fa",
        stat: "#eff4ff",
      },
      borderRadius: {
        sm: "4px",
        md: "6px",
        lg: "8px",
        xl: "12px",
        "2xl": "16px",
      },
      fontFamily: {
        sans: ['"PingFang SC"', '"Microsoft YaHei"', '"Hiragino Sans GB"', '"Helvetica Neue"', "Arial", "sans-serif"],
      },
      keyframes: {
        "fade-in": {
          from: { opacity: "0", transform: "translateY(-4px)" },
          to: { opacity: "1", transform: "translateY(0)" },
        },
        "fade-out": {
          from: { opacity: "1", transform: "translateY(0)" },
          to: { opacity: "0", transform: "translateY(-4px)" },
        },
      },
      animation: {
        "fade-in": "fade-in 0.2s ease-out",
        "fade-out": "fade-out 0.2s ease-out",
      },
    },
  },
  plugins: [require("tailwindcss-animate")],
}

export default config
