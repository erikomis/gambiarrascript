// Link compartilhavel do playground: o codigo vai inteiro no hash da URL
// (nada sai do navegador). Formatos:
//   #c=<base64url(deflate-raw(utf8))>  quando tem CompressionStream
//   #t=<base64url(utf8)>               fallback sem compressao
// A entrada (stdin) vai junto, opcional, separada por `&` (base64url nunca tem
// `&`), com a mesma codificacao:
//   #c=...&e=<base64url(deflate-raw(utf8))>   ou   &u=<base64url(utf8)>
// Link so com `c=`/`t=` (ex.: os botoes da doc) continua valendo: entrada vazia.

function paraBase64Url(bytes: Uint8Array): string {
  let bin = "";
  // em blocos pra nao estourar a pilha do String.fromCharCode(...)
  for (let i = 0; i < bytes.length; i += 0x8000) {
    bin += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  }
  return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function deBase64Url(s: string): Uint8Array {
  const b64 = s.replace(/-/g, "+").replace(/_/g, "/");
  const bin = atob(b64 + "=".repeat((4 - (b64.length % 4)) % 4));
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return bytes;
}

async function passaPor(
  bytes: Uint8Array,
  transformador: CompressionStream | DecompressionStream
): Promise<Uint8Array> {
  const stream = new Blob([bytes as BlobPart]).stream().pipeThrough(transformador);
  return new Uint8Array(await new Response(stream).arrayBuffer());
}

// comprime (ou nao, se o navegador nao deixar) e devolve o valor em base64url
async function empacota(
  texto: string
): Promise<{ comprimido: boolean; valor: string }> {
  const bytes = new TextEncoder().encode(texto);
  if (typeof CompressionStream !== "undefined") {
    try {
      const comp = await passaPor(bytes, new CompressionStream("deflate-raw"));
      return { comprimido: true, valor: paraBase64Url(comp) };
    } catch {
      // navegador sem deflate-raw: cai no texto puro
    }
  }
  return { comprimido: false, valor: paraBase64Url(bytes) };
}

async function desempacota(valor: string, comprimido: boolean): Promise<string> {
  const bytes = deBase64Url(valor);
  if (!comprimido) return new TextDecoder().decode(bytes);
  if (typeof DecompressionStream === "undefined") {
    throw new Error("navegador sem DecompressionStream");
  }
  return new TextDecoder().decode(
    await passaPor(bytes, new DecompressionStream("deflate-raw"))
  );
}

// devolve o hash (sem o `#`) que representa o codigo e, se tiver, a entrada
export async function codificar(codigo: string, entrada = ""): Promise<string> {
  const c = await empacota(codigo);
  let hash = (c.comprimido ? "c=" : "t=") + c.valor;
  if (entrada) {
    const e = await empacota(entrada);
    hash += (e.comprimido ? "&e=" : "&u=") + e.valor;
  }
  return hash;
}

export interface LinkPlayground {
  codigo: string;
  entrada: string;
}

// le codigo + entrada de um hash (com ou sem `#`); null se nao for um link do
// playground
export async function decodificarLink(hash: string): Promise<LinkPlayground | null> {
  const partes = new Map<string, string>();
  for (const parte of hash.replace(/^#/, "").split("&")) {
    const i = parte.indexOf("=");
    if (i > 0) partes.set(parte.slice(0, i), parte.slice(i + 1));
  }
  try {
    let codigo: string;
    if (partes.has("c")) codigo = await desempacota(partes.get("c")!, true);
    else if (partes.has("t")) codigo = await desempacota(partes.get("t")!, false);
    else return null;

    let entrada = "";
    if (partes.has("e")) entrada = await desempacota(partes.get("e")!, true);
    else if (partes.has("u")) entrada = await desempacota(partes.get("u")!, false);
    return { codigo, entrada };
  } catch {
    // link quebrado/truncado
  }
  return null;
}

// so o codigo do hash (compat); null se nao for um link do playground
export async function decodificar(hash: string): Promise<string | null> {
  return (await decodificarLink(hash))?.codigo ?? null;
}
