// Worker do playground: carrega o runtime Go (wasm_exec.js + gs.wasm) e roda
// o codigo fora da thread principal. Assim um `enquanto deu_bom` sem fim so
// trava o worker — a pagina segue de boa e o botao Parar mata ele.
//
// Arquivo estatico de proposito (fica em public/, fora do bundle): o Next com
// `output: "export"` + basePath so copia ele pra out/ e pronto. Os caminhos
// abaixo sao relativos a URL do proprio worker, entao funcionam com ou sem
// basePath (ex.: /gambiarrascript/gs-worker.js -> /gambiarrascript/gs.wasm).
//
// Protocolo (postMessage):
//   pagina -> worker: {tipo: "init", modulo?}     modulo = WebAssembly.Module ja compilado
//                     {tipo: "rodar", id, codigo, entrada?}  entrada = stdin do programa
//   worker -> pagina: {tipo: "modulo", modulo}    pra pagina reaproveitar no proximo worker
//                     {tipo: "pronto"}
//                     {tipo: "erro-carga", mensagem}
//                     {tipo: "saida", id, texto}  pedaco de saida ao vivo
//                     {tipo: "fim", id, saida, erros}
//                     {tipo: "morreu"}            runtime Go saiu (panic): precisa de worker novo

/* eslint-disable no-restricted-globals */
"use strict";

const url = (arquivo) => new URL(arquivo, self.location.href).toString();

importScripts(url("wasm_exec.js"));

// baixa o gs.wasm.gz e descomprime no navegador; devolve null se nao rolar
async function compilarGz() {
  if (typeof DecompressionStream === "undefined") return null;
  try {
    const res = await fetch(url("gs.wasm.gz"));
    if (!res.ok || !res.body) return null;
    const corpo = res.body.pipeThrough(new DecompressionStream("gzip"));
    const wasm = new Response(corpo, {
      headers: { "Content-Type": "application/wasm" },
    });
    return await WebAssembly.compileStreaming(wasm);
  } catch (e) {
    // ex.: servidor que ja mandou com Content-Encoding: gzip (o navegador
    // descomprimiu sozinho e o DecompressionStream engasga) — cai no cru.
    console.warn("gs.wasm.gz falhou, usando gs.wasm:", e);
    return null;
  }
}

async function compilarCru() {
  const res = await fetch(url("gs.wasm"));
  if (!res.ok) throw new Error(`falhou baixar gs.wasm (HTTP ${res.status})`);
  try {
    return await WebAssembly.compileStreaming(res.clone());
  } catch {
    // servidor sem Content-Type application/wasm: compila do buffer
    return await WebAssembly.compile(await res.arrayBuffer());
  }
}

let go = null;

async function carregar(moduloPronto) {
  let modulo = moduloPronto;
  if (!(modulo instanceof WebAssembly.Module)) {
    modulo = (await compilarGz()) ?? (await compilarCru());
    try {
      self.postMessage({ tipo: "modulo", modulo });
    } catch {
      // navegador que nao clona WebAssembly.Module: o proximo worker baixa de novo
    }
  }
  go = new self.Go();
  const instancia = await WebAssembly.instantiate(modulo, go.importObject);
  go.run(instancia); // nao resolve nunca: o main do Go fica parado em <-c
  if (!self.GambiarraScript || !self.GambiarraScript.evaluate) {
    throw new Error("gs.wasm nao expos GambiarraScript.evaluate");
  }
}

// junta os pedacos de saida e manda no maximo a cada ~50ms; o evaluate e
// sincrono, entao nao da pra usar timer — confere o relogio a cada escrita.
const INTERVALO_MS = 50;
const LIMITE_PEDACO = 64 * 1024;

function rodar(id, codigo, entrada) {
  let pendente = "";
  let ultimoEnvio = performance.now();
  const despeja = () => {
    if (pendente) {
      self.postMessage({ tipo: "saida", id, texto: pendente });
      pendente = "";
    }
    ultimoEnvio = performance.now();
  };
  const onSaida = (texto) => {
    pendente += texto;
    if (
      pendente.length >= LIMITE_PEDACO ||
      performance.now() - ultimoEnvio >= INTERVALO_MS
    ) {
      despeja();
    }
  };

  let saida = "";
  let erros = "";
  try {
    const res = self.GambiarraScript.evaluate(codigo, onSaida, entrada ?? "");
    saida = res.saida ?? "";
    erros = res.erros ?? "";
  } catch (e) {
    erros = String((e && e.message) || e);
  }
  despeja();
  self.postMessage({ tipo: "fim", id, saida, erros });
  // panic no Go derruba o runtime inteiro; avisa pra pagina trocar de worker
  if (go && go.exited) self.postMessage({ tipo: "morreu" });
}

let pronto = null;

self.onmessage = (ev) => {
  const msg = ev.data || {};
  if (msg.tipo === "init") {
    pronto = carregar(msg.modulo).then(
      () => self.postMessage({ tipo: "pronto" }),
      (e) => {
        self.postMessage({
          tipo: "erro-carga",
          mensagem: String((e && e.message) || e),
        });
        throw e;
      }
    );
    pronto.catch(() => {});
  } else if (msg.tipo === "rodar") {
    if (!pronto) {
      self.postMessage({ tipo: "fim", id: msg.id, saida: "", erros: "worker nao iniciado" });
      return;
    }
    pronto.then(
      () => rodar(msg.id, msg.codigo, msg.entrada),
      (e) =>
        self.postMessage({
          tipo: "fim",
          id: msg.id,
          saida: "",
          erros: String((e && e.message) || e),
        })
    );
  }
};
