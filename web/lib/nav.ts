import type { NavOptions } from "fumadocs-ui/layouts/shared";

export const navDefaults: NavOptions = {
  title: "GambiarraScript",
  url: "/",
};

export const navLinks = [
  { text: "Docs", url: "/docs", active: "nested-url" as const },
  { text: "Playground", url: "/playground" },
  {
    text: "GitHub",
    url: "https://github.com/erikomis/gambiarrascript",
    external: true,
  },
];