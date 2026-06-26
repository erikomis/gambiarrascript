// Carrega o runtime WASM do GambiarraScript (gs.wasm) uma unica vez por
// aba e expoe uma funcao `evaluate` pra usar no playground.
//
// O arquivo `wasm_exec.js` foi copiado de `lib/wasm` do Go; ele define
// `Go` no escopo global. O bundle do Next nao o inclui, entao injetamos
// via <script> na tag <head> (ver Playground).

declare global {
  interface Window {
    Go?: any;
    GambiarraScript?: { evaluate: (code: string) => { saida: string; erros: string } };
    __gsWasmReady?: { ready: boolean };
  }
}

let loadPromise: Promise<void> | null = null;

function loadScript(src: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const existing = document.querySelector(`script[data-src="${src}"]`);
    if (existing) {
      resolve();
      return;
    }
    const s = document.createElement("script");
    s.src = src;
    s.async = true;
    s.dataset.src = src;
    s.onload = () => resolve();
    s.onerror = () => reject(new Error(`falhou carregar ${src}`));
    document.head.appendChild(s);
  });
}

export function ensureWasmRuntime(): Promise<void> {
  if (typeof window === "undefined") {
    return Promise.reject(new Error("wasm so roda no navegador"));
  }
  if (!loadPromise) {
    loadPromise = (async () => {
      await loadScript("/wasm_exec.js");
      if (!window.Go) {
        throw new Error("Go runtime nao encontrado apos carregar wasm_exec.js");
      }
      const go = new window.Go();
      let instance: WebAssembly.Instance;
      try {
        // caminho rapido: streaming quando o navegador suporta
        const streaming = await WebAssembly.instantiateStreaming(
          fetch("/gs.wasm"),
          go.importObject
        );
        instance = streaming.instance;
      } catch {
        // fallback: baixa o binario inteiro e instancia do buffer
        const res = await fetch("/gs.wasm");
        const buf = await res.arrayBuffer();
        const instantiated = await WebAssembly.instantiate(
          buf,
          go.importObject
        );
        instance =
          instantiated instanceof WebAssembly.Instance
            ? instantiated
            : instantiated.instance;
      }
      go.run(instance);
      // GambiarraScript.evaluate deve ter sido registrado por cmd/wasm
      if (!window.GambiarraScript?.evaluate) {
        throw new Error("gs.wasm nao expôs GambiarraScript.evaluate");
      }
    })().catch((err) => {
      loadPromise = null; // permite retry
      throw err;
    });
  }
  return loadPromise;
}

export interface EvalResult {
  saida: string;
  erros: string;
}

export async function evaluate(code: string): Promise<EvalResult> {
  await ensureWasmRuntime();
  return window.GambiarraScript!.evaluate(code);
}