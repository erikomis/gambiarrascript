// Link compartilhavel do playground: o codigo vai inteiro no hash da URL
// (nada sai do navegador). Formatos:
//   #c=<base64url(deflate-raw(utf8))>  quando tem CompressionStream
//   #t=<base64url(utf8)>               fallback sem compressao

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

// devolve o hash (sem o `#`) que representa o codigo
export async function codificar(codigo: string): Promise<string> {
  const bytes = new TextEncoder().encode(codigo);
  if (typeof CompressionStream !== "undefined") {
    try {
      const comp = await passaPor(bytes, new CompressionStream("deflate-raw"));
      return "c=" + paraBase64Url(comp);
    } catch {
      // navegador sem deflate-raw: cai no texto puro
    }
  }
  return "t=" + paraBase64Url(bytes);
}

// le o codigo de um hash (com ou sem `#`); null se nao for um link do playground
export async function decodificar(hash: string): Promise<string | null> {
  const h = hash.replace(/^#/, "");
  try {
    if (h.startsWith("c=")) {
      if (typeof DecompressionStream === "undefined") return null;
      const bytes = await passaPor(
        deBase64Url(h.slice(2)),
        new DecompressionStream("deflate-raw")
      );
      return new TextDecoder().decode(bytes);
    }
    if (h.startsWith("t=")) {
      return new TextDecoder().decode(deBase64Url(h.slice(2)));
    }
  } catch {
    // link quebrado/truncado
  }
  return null;
}
