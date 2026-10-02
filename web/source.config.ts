import { defineDocs, defineConfig } from "fumadocs-mdx/config";
import {
  rehypeCodeDefaultOptions,
  type RehypeCodeOptions,
} from "fumadocs-core/mdx-plugins";
// O mesmo grammar TextMate da extensao do VSCode — fonte unica de verdade pro
// highlight. Importado (e nao copiado) pra docs e editor nunca divergirem.
import gambiarraGrammar from "../editors/vscode/syntaxes/gambiarrascript.tmLanguage.json";
import { transformerPlayground } from "./lib/transformer-playground";

type LinguagemShiki = NonNullable<RehypeCodeOptions["langs"]>[number];

// Registra o grammar como linguagem custom do shiki: blocos ```gambiarrascript
// e ```gs usam ele.
const gambiarrascript = {
  ...gambiarraGrammar,
  name: "gambiarrascript",
  displayName: "GambiarraScript",
  aliases: ["gs"],
} as unknown as LinguagemShiki;

export const docs = defineDocs({
  dir: "content/docs",
});

export default defineConfig({
  mdxOptions: {
    rehypeCodeOptions: {
      ...rehypeCodeDefaultOptions,
      langs: [gambiarrascript, "bash", "json"],
      // blocos gambiarrascript ganham o link "Rodar no playground"
      transformers: [
        ...(rehypeCodeDefaultOptions.transformers ?? []),
        transformerPlayground(),
      ],
      // lingua desconhecida vira texto puro (em vez de quebrar o build)
      fallbackLanguage: "text",
    },
  },
});
