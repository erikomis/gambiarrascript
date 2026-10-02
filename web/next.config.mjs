import { createMDX } from "fumadocs-mdx/next";

const withMDX = createMDX();

// No GitHub Pages o site fica em https://<user>.github.io/gambiarrascript/,
// entao o CI passa NEXT_PUBLIC_BASE_PATH=/gambiarrascript. Local fica vazio.
const basePath = process.env.NEXT_PUBLIC_BASE_PATH || "";

/** @type {import('next').NextConfig} */
const config = {
  reactStrictMode: true,
  // site 100% estatico (out/) — da pra hospedar em qualquer CDN/GitHub Pages
  output: "export",
  basePath,
  trailingSlash: true,
  images: { unoptimized: true },
  typescript: {
    ignoreBuildErrors: false,
  },
  // gs.wasm eh grande (~14MB) — garante que nao seja processado nem
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
