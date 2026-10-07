import type { ComponentProps } from "react";
import defaultMdxComponents from "fumadocs-ui/mdx";
import { CodeBlock, Pre } from "fumadocs-ui/components/codeblock";
import { Callout } from "fumadocs-ui/components/callout";
import { Steps, Step } from "fumadocs-ui/components/steps";
import { TypeTable } from "fumadocs-ui/components/type-table";
import { Accordions, Accordion } from "fumadocs-ui/components/accordion";
import { CodeBlockPlayground } from "./CodeBlockPlayground";
import { decodificarNode } from "@/lib/compartilhar-node";

// mesmo basePath do next.config.mjs (GitHub Pages: /gambiarrascript). O link
// eh um <a> comum (nao next/link): carrega a pagina do playground do zero, que
// le o codigo do hash no mount.
const basePath = process.env.NEXT_PUBLIC_BASE_PATH || "";

// builtins que dependem de rede/arquivo/banco: o botao aparece igual (da pra
// estudar/editar o codigo), mas a dica avisa que no navegador nao roda
const SO_NO_NATIVO =
  /\b(escuta|rota|busca|conecta|consulta|executa|le_arquivo|escreve_arquivo)\s*\(/;

// builtins que leem stdin: rodam no playground, a entrada vem da caixa
// "Entrada (stdin)" de la
const LE_ENTRADA = /\b(pergunta|le_linhas|le_tudo)\s*\(/;

type PreProps = ComponentProps<"pre"> & {
  "data-playground"?: string;
  title?: string;
  icon?: string;
};

export function getMDXComponents(opcoes: { ingles?: boolean } = {}) {
  const rotulo = opcoes.ingles ? "Run in playground" : "Rodar no playground";

  function PreComPlayground({ "data-playground": hash, ...props }: PreProps) {
    const conteudo = <Pre>{props.children}</Pre>;
    // data-playground vem do transformer do shiki (lib/transformer-playground.ts),
    // so em blocos gambiarrascript/gs
    if (!hash) return <CodeBlock {...props}>{conteudo}</CodeBlock>;

    return (
      <CodeBlockPlayground
        {...props}
        href={`${basePath}/playground/#${hash}`}
        rotulo={rotulo}
        dica={dicaPara(hash, opcoes.ingles)}
      >
        {conteudo}
      </CodeBlockPlayground>
    );
  }

  return {
    ...defaultMdxComponents,
    pre: PreComPlayground,
    Callout,
    Steps,
    Step,
    TypeTable,
    Accordions,
    Accordion,
  };
}

function dicaPara(hash: string, ingles?: boolean): string {
  const codigo = decodificarNode(hash) ?? "";
  if (SO_NO_NATIVO.test(codigo)) {
    return ingles
      ? "Open in the playground (network/file/db builtins don't run in the browser)"
      : "Abre no playground (rede/arquivo/banco nao rodam no navegador)";
  }
  if (LE_ENTRADA.test(codigo)) {
    return ingles
      ? "Open in the playground (type the input in the Entrada (stdin) box)"
      : "Abre no playground (a entrada vai na caixa Entrada (stdin))";
  }
  return ingles ? "Open this code in the playground" : "Abre esse codigo no playground";
}
