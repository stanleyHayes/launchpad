import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  transpilePackages: ["@launchpad/ui", "@launchpad/api-client"],
};

export default nextConfig;
