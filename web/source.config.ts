import { defineDocs, defineConfig } from "fumadocs-mdx/config";

export const docs = defineDocs({
  dir: "content/docs",
});

export default defineConfig({
  mdxOptions: {
    // GambiarraScript ainda nao tem grammar propria no shiki;
    // emprestamos o highlight de TSX enquanto nao temos uma.
    rehypeCodeOptions: {
      langAlias: { gambiarrascript: "tsx" },
      langs: ["tsx", "typescript", "bash", "json"],
      fallbackLanguage: "tsx",
    } as never,
  },
});