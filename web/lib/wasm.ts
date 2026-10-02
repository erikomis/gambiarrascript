// Cliente do runtime WASM do GambiarraScript. O gs.wasm roda dentro de um Web
// Worker (public/gs-worker.js), nunca na thread da pagina: um laco infinito
// trava so o worker, e "parar" e literalmente `worker.terminate()` + um worker
// novo.
//
// O worker e um arquivo estatico em public/ (nao passa pelo bundler). Com
// `output: "export"` o Next so copia ele pra out/, e aqui a gente prefixa com
// o basePath. Dentro do worker os caminhos do wasm sao relativos a URL dele.

// prefixo do deploy (ex.: /gambiarrascript no GitHub Pages); vazio em dev.
const base = process.env.NEXT_PUBLIC_BASE_PATH ?? "";

// tempo maximo de uma execucao antes de matar o worker
export const TIMEOUT_PADRAO_MS = 10_000;

export type EstadoRuntime =
  | { tipo: "carregando" }
  | { tipo: "pronto" }
  | { tipo: "erro"; mensagem: string };

export interface EvalResult {
  saida: string;
  erros: string;
  // preenchido quando a execucao nao terminou sozinha
  interrompido?: "parado" | "timeout";
}

interface Execucao {
  id: number;
  resolver: (r: EvalResult) => void;
  onSaida?: (texto: string) => void;
  timer: ReturnType<typeof setTimeout>;
}

type MsgWorker =
  | { tipo: "modulo"; modulo: WebAssembly.Module }
  | { tipo: "pronto" }
  | { tipo: "erro-carga"; mensagem: string }
  | { tipo: "saida"; id: number; texto: string }
  | { tipo: "fim"; id: number; saida: string; erros: string }
  | { tipo: "morreu" };

// modulo ja compilado, guardado pra o proximo worker nao baixar/compilar o
// gs.wasm de novo depois de um Parar.
let moduloCache: WebAssembly.Module | null = null;

export class RuntimeGS {
  private worker: Worker | null = null;
  private execucao: Execucao | null = null;
  private proximoId = 1;
  private destruido = false;

  constructor(private onEstado: (e: EstadoRuntime) => void) {
    this.iniciarWorker();
  }

  private iniciarWorker() {
    this.onEstado({ tipo: "carregando" });
    const w = new Worker(`${base}/gs-worker.js`);
    this.worker = w;
    w.onmessage = (ev: MessageEvent<MsgWorker>) => {
      if (this.worker !== w) return; // worker velho, ja descartado
      const msg = ev.data;
      switch (msg.tipo) {
        case "modulo":
          moduloCache = msg.modulo;
          break;
        case "pronto":
          this.onEstado({ tipo: "pronto" });
          break;
        case "erro-carga":
          this.onEstado({ tipo: "erro", mensagem: msg.mensagem });
          break;
        case "saida":
          if (this.execucao?.id === msg.id) this.execucao.onSaida?.(msg.texto);
          break;
        case "fim":
          if (this.execucao?.id === msg.id) {
            this.encerrar({ saida: msg.saida, erros: msg.erros });
          }
          break;
        case "morreu":
          // o Go saiu (panic): esse worker nao roda mais nada, troca por outro
          this.parar();
          break;
      }
    };
    w.onerror = (ev) => {
      if (this.worker !== w) return;
      ev.preventDefault();
      const mensagem = ev.message || "o worker do runtime caiu";
      this.onEstado({ tipo: "erro", mensagem });
      this.encerrar({ saida: "", erros: mensagem });
    };
    w.postMessage({ tipo: "init", modulo: moduloCache ?? undefined });
  }

  private encerrar(r: EvalResult) {
    const ex = this.execucao;
    if (!ex) return;
    clearTimeout(ex.timer);
    this.execucao = null;
    ex.resolver(r);
  }

  get rodando() {
    return this.execucao !== null;
  }

  // roda o codigo; `onSaida` recebe a saida em pedacos, ao vivo
  rodar(
    codigo: string,
    opts: { onSaida?: (texto: string) => void; timeoutMs?: number } = {}
  ): Promise<EvalResult> {
    if (this.destruido || !this.worker) {
      return Promise.resolve({ saida: "", erros: "runtime encerrado" });
    }
    if (this.execucao) this.parar();
    const id = this.proximoId++;
    return new Promise<EvalResult>((resolver) => {
      const timer = setTimeout(
        () => this.parar("timeout"),
        opts.timeoutMs ?? TIMEOUT_PADRAO_MS
      );
      this.execucao = { id, resolver, onSaida: opts.onSaida, timer };
      this.worker!.postMessage({ tipo: "rodar", id, codigo });
    });
  }

  // mata o worker (e o que estiver rodando nele) e sobe um novo
  parar(motivo: "parado" | "timeout" = "parado") {
    this.worker?.terminate();
    this.worker = null;
    this.encerrar({ saida: "", erros: "", interrompido: motivo });
    if (!this.destruido) this.iniciarWorker();
  }

  destruir() {
    this.destruido = true;
    this.worker?.terminate();
    this.worker = null;
    this.encerrar({ saida: "", erros: "", interrompido: "parado" });
  }
}
