import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  allowedDevOrigins: ["47.107.124.45"],
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: "http://127.0.0.1:8088/api/:path*",
      },
    ];
  },
};

export default nextConfig;
