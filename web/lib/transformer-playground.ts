// Transformer do shiki (roda no build, dentro do rehype-code do fumadocs):
// todo bloco ```gambiarrascript / ```gs ganha o atributo `data-playground`
// com o hash do playground (`c=...`) ja calculado a partir do codigo cru.
// O componente `pre` do MDX (components/mdx.tsx) le esse atributo e mostra o
// botao "Rodar no playground".
//
// Pra esconder o botao num bloco especifico: ```gambiarrascript sem-playground
import type { RehypeCodeOptions } from "fumadocs-core/mdx-plugins";
import { codificarNode } from "./compartilhar-node";

type Transformer = NonNullable<RehypeCodeOptions["transformers"]>[number];

const LINGUAS = new Set(["gambiarrascript", "gs"]);
const OPT_OUT = /(^|\s)sem-playground(\s|$)/;

export function transformerPlayground(): Transformer {
  return {
    name: "gambiarrascript:playground",
    pre(node) {
      if (!LINGUAS.has(this.options.lang)) return;
      const meta = this.options.meta?.__raw ?? "";
      if (OPT_OUT.test(meta)) return;
      // this.source eh o codigo cru do bloco (sem o \n final, que o
      // fumadocs tira antes de mandar pro shiki)
      const codigo = this.source;
      if (!codigo.trim()) return;
      node.properties["data-playground"] = codificarNode(codigo + "\n");
    },
  };
}
