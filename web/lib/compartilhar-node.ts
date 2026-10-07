// Versao Node (build) do codificador de lib/compartilhar.ts. Gera o mesmo
// formato `c=<base64url(deflate-raw(utf8))>` que o playground le com
// decodificar() — deflate-raw eh um formato padrao (RFC 1951), entao o que o
// zlib do Node comprime o DecompressionStream do navegador abre igualzinho.
// Usado em tempo de build (transformer do shiki), zero JS no cliente. A doc so
// manda codigo; a entrada (`&e=`/`&u=`, ver lib/compartilhar.ts) e ignorada
// aqui na leitura.
import { deflateRawSync, inflateRawSync } from "node:zlib";

// devolve o hash (sem o `#`) que representa o codigo
export function codificarNode(codigo: string): string {
  const comp = deflateRawSync(Buffer.from(codigo, "utf8"), { level: 9 });
  // base64url do Node ja sai sem `=` de padding, igual ao paraBase64Url()
  return "c=" + comp.toString("base64url");
}

// inverso, so pra conferencia/teste (o site usa o decodificar() do navegador)
export function decodificarNode(hash: string): string | null {
  // so a parte do codigo; base64url nunca tem `&`
  const h = hash.replace(/^#/, "").split("&")[0];
  try {
    if (h.startsWith("c=")) {
      return inflateRawSync(Buffer.from(h.slice(2), "base64url")).toString("utf8");
    }
    if (h.startsWith("t=")) {
      return Buffer.from(h.slice(2), "base64url").toString("utf8");
    }
  } catch {
    // link quebrado/truncado
  }
  return null;
}
