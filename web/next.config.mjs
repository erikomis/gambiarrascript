import { createMDX } from "fumadocs-mdx/next";

const withMDX = createMDX();

/** @type {import('next').NextConfig} */
const config = {
  reactStrictMode: true,
  typescript: {
    ignoreBuildErrors: false,
  },
  // gs.wasm eh grande (~10MB) — garante que nao seja processado nem
  // bloqueado por outros otimizadores.
  webpack: (config) => {
    config.module.rules.push({
      test: /\.(wasm)$/,
      type: "asset/resource",
    });
    return config;
  },
};

export default withMDX(config);