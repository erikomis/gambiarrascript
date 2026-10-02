"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import dynamic from "next/dynamic";
import { Prec } from "@codemirror/state";
import { keymap } from "@codemirror/view";
import { buttonVariants } from "fumadocs-ui/components/ui/button";
import { RuntimeGS, TIMEOUT_PADRAO_MS, type EstadoRuntime } from "@/lib/wasm";
import { gambiarraScript } from "@/lib/gs-language";
import { codificar, decodificar } from "@/lib/compartilhar";


const defaultCode = `# o classico, do jeito gambiarra
mostra "Salve, tropa!"

bota nome = "Erik"
bota idade = 25

se_colar idade >= 18
    mostra nome + " pode entrar"
se_nao_colar
    mostra "volta daqui a pouco"
acabou_finalmente

gambiarra dobra(n)
    funciona n * 2
acabou_finalmente

pra_cada i de 1 ate 3
    mostra "dobro de " + i + " = " + dobra(i)
acabou_finalmente
`;

const examples: { nome: string; codigo: string }[] = [
  { nome: "Salve, tropa", codigo: defaultCode },
  {
    nome: "FizzBuzz",
    codigo: `pra_cada i de 1 ate 20
    se_colar i % 15 == 0
        mostra "FizzBuzz"
    se_nao_colar se_colar i % 3 == 0
        mostra "Fizz"
    se_nao_colar se_colar i % 5 == 0
        mostra "Buzz"
    se_nao_colar
        mostra i
    acabou_finalmente
acabou_finalmente
`,
  },
  {
    nome: "Lista e dicionario",
    codigo: `bota frutas = ["abacaxi", "goiaba", "caju"]
bota frutas[1] = "jambo"
mostra frutas

bota pessoa = {"nome": "Erik", "idade": 25}
mostra pessoa["nome"] + " tem " + pessoa["idade"] + " anos"

pra_cada fruta em frutas
    mostra "hoje tem: " + fruta
acabou_finalmente
`,
  },
  {
    nome: "Arruma/Quebrou",
    codigo: `arruma
    bota resultado = 10 / 0
quebrou erro
    mostra "deu ruim, parca: " + erro
acabou_finalmente
`,
  },
  {
    nome: "JSON",
    codigo: `bota dados = de_json(\`{"nome": "Erik", "tags": ["go", "gs"]}\`)
mostra dados["nome"]
mostra dados["tags"][0]
mostra pra_json({"ok": deu_bom, "n": 42})
`,
  },
  {
    nome: "Crava e matematica",
    codigo: `crava TAXA = 0.1
# tira o # da linha abaixo pra ver o erro de constante cravada
# bota TAXA = 0.2

mostra 2 ** 10                 # potencia
mostra 2 ** 3 ** 2             # associa pela direita: 512

bota r = 3
mostra "area: " + formata("%.2f", pi * r ** 2)
mostra "seno de 90: " + seno(pi / 2)

bota capital = 1000
capital *= (1 + TAXA) ** 2
mostra "capital em 2 anos: " + formata("%.2f", capital)
`,
  },
  {
    nome: "Bora (paralelo)",
    codigo: `gambiarra demora(n)
    bota out = 0
    pra_cada i de 1 ate n
        bota out = out + i
    acabou_finalmente
    funciona out
acabou_finalmente

bota f1 = bora demora(100)
bota f2 = bora demora(1000)
mostra "rodando em paralelo..."
mostra espera(f1)
mostra espera(f2)
`,
  },
  {
    nome: "Cano (canal)",
    codigo: `gambiarra produtor(c)
    pra_cada i de 1 ate 3
        mostra "produzi " + i
        envia(c, i)
    acabou_finalmente
    fecha(c)
acabou_finalmente

bota c = cano(3)
bora produtor(c)

bota soma = 0
enquanto deu_bom
    bota v = recebe(c)
    se_colar v == nada
        vaza
    acabou_finalmente
    bota soma = soma + v
acabou_finalmente
mostra "soma: " + soma
`,
  },
];


// Editor Monaco seria pesado; CodeMirror 6 eh suficiente e leve.
// Carregado dinamicamente para nao inflar o bundle inicial do SSR.
const CodeMirror = dynamic(
  () => import("@uiw/react-codemirror"),
  { ssr: false, loading: () => <div className="h-[480px] animate-pulse bg-fd-muted" /> }
);

// saida maxima mantida na tela; um `mostra` dentro de laco infinito nao pode
// derrubar a aba de tanto texto
const LIMITE_SAIDA = 200_000;

export default function Playground() {
  const [code, setCode] = useState(defaultCode);
  const [output, setOutput] = useState("");
  const [erro, setErro] = useState("");
  const [aviso, setAviso] = useState("");
  const [estado, setEstado] = useState<EstadoRuntime>({ tipo: "carregando" });
  const [rodando, setRodando] = useState(false);
  const [feedback, setFeedback] = useState("");
  const runtimeRef = useRef<RuntimeGS | null>(null);
  const saidaRef = useRef("");
  const cortadaRef = useRef(false);
  const outRef = useRef<HTMLPreElement>(null);

  const pronto = estado.tipo === "pronto";

  // sobe o worker com o runtime WASM ao montar
  useEffect(() => {
    const rt = new RuntimeGS(setEstado);
    runtimeRef.current = rt;
    return () => {
      rt.destruir();
      runtimeRef.current = null;
    };
  }, []);

  // link compartilhado: #c=... no hash vira o codigo do editor
  useEffect(() => {
    const carregaDoHash = () => {
      if (!window.location.hash) return;
      decodificar(window.location.hash).then((c) => {
        if (c !== null) setCode(c);
      });
    };
    carregaDoHash();
    window.addEventListener("hashchange", carregaDoHash);
    return () => window.removeEventListener("hashchange", carregaDoHash);
  }, []);

  const anexaSaida = useCallback((texto: string) => {
    let s = saidaRef.current + texto;
    if (s.length > LIMITE_SAIDA) {
      s = s.slice(s.length - LIMITE_SAIDA);
      cortadaRef.current = true;
    }
    saidaRef.current = s;
    setOutput(s);
  }, []);

  const rodar = useCallback(async () => {
    const rt = runtimeRef.current;
    if (!rt || !pronto || rodando) return;
    saidaRef.current = "";
    cortadaRef.current = false;
    setOutput("");
    setErro("");
    setAviso("");
    setRodando(true);
    try {
      const res = await rt.rodar(code, { onSaida: anexaSaida });
      if (res.saida) anexaSaida(res.saida);
      setErro(res.erros);
      if (res.interrompido === "timeout") {
        setAviso(
          `execucao interrompida: passou de ${TIMEOUT_PADRAO_MS / 1000}s (laco infinito?)`
        );
      } else if (res.interrompido === "parado") {
        setAviso("execucao interrompida");
      }
    } finally {
      setRodando(false);
    }
  }, [code, pronto, rodando, anexaSaida]);

  const parar = useCallback(() => {
    runtimeRef.current?.parar();
  }, []);

  const compartilhar = useCallback(async () => {
    const hash = await codificar(code);
    // replaceState nao dispara hashchange, entao o editor nao recarrega
    window.history.replaceState(null, "", `#${hash}`);
    const link = window.location.href;
    try {
      await navigator.clipboard.writeText(link);
      setFeedback("link copiado!");
    } catch {
      setFeedback("link pronto na barra de endereco");
    }
    setTimeout(() => setFeedback(""), 2500);
  }, [code]);

  // Ctrl/Cmd+Enter roda — dentro do editor (keymap com prioridade maxima,
  // senao o CodeMirror insere linha) e fora dele (listener na janela)
  const rodarRef = useRef(rodar);
  rodarRef.current = rodar;
  const extensoes = useMemo(
    () => [
      gambiarraScript,
      Prec.highest(
        keymap.of([
          {
            key: "Mod-Enter",
            run: () => {
              void rodarRef.current();
              return true;
            },
          },
        ])
      ),
    ],
    []
  );
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.key !== "Enter" || !(e.ctrlKey || e.metaKey)) return;
      e.preventDefault();
      void rodarRef.current();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  // acompanha o fim da saida enquanto ela chega
  useEffect(() => {
    const el = outRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [output, erro, aviso]);

  const rotuloRodar =
    estado.tipo === "carregando"
      ? "carregando runtime..."
      : rodando
        ? "rodando..."
        : "Rodar";

  return (
    <main className="mx-auto w-full max-w-6xl px-6 py-10">
      <div className="mb-6">
        <h1 className="text-3xl font-semibold">Playground</h1>
        <p className="text-fd-muted-foreground">
          Roda o GambiarraScript direto no navegador via WASM. Sem servidor,
          sem segredo — so voce e tua gambiarra.
        </p>
      </div>

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <button
          onClick={() => void rodar()}
          disabled={!pronto || rodando}
          title="Ctrl/Cmd + Enter"
          className={buttonVariants({ color: "primary" })}
        >
          {rotuloRodar}
        </button>
        <button
          onClick={parar}
          disabled={!rodando}
          className={buttonVariants({ color: "outline" })}
        >
          Parar
        </button>
        <button
          onClick={() => {
            setCode("");
            saidaRef.current = "";
            setOutput("");
            setErro("");
            setAviso("");
          }}
          className={buttonVariants({ color: "outline" })}
        >
          Limpar
        </button>
        <button
          onClick={() => void compartilhar()}
          className={buttonVariants({ color: "outline" })}
        >
          Compartilhar
        </button>
        {feedback && (
          <span role="status" className="text-sm text-fd-muted-foreground">
            {feedback}
          </span>
        )}
        <span className="ml-auto text-sm text-fd-muted-foreground">
          Exemplos:
        </span>
        {examples.map((ex) => (
          <button
            key={ex.nome}
            onClick={() => setCode(ex.codigo)}
            className="rounded-md border border-fd-foreground/15 px-2 py-1 text-sm hover:bg-fd-muted"
          >
            {ex.nome}
          </button>
        ))}
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="overflow-hidden rounded-lg border border-fd-foreground/15">
          <CodeMirror
            value={code}
            height="480px"
            onChange={(v) => setCode(v)}
            theme="dark"
            extensions={extensoes}
            basicSetup={{
              lineNumbers: true,
              highlightActiveLine: true,
              foldGutter: false,
            }}
          />
        </div>

        <div className="min-h-[480px] rounded-lg border border-fd-foreground/15 bg-fd-muted/30 p-3">
          <div className="mb-2 flex items-center gap-2 text-xs uppercase tracking-wide text-fd-muted-foreground">
            saida
            <span className="ml-auto normal-case tracking-normal">
              Ctrl/Cmd + Enter roda
            </span>
          </div>
          <pre
            ref={outRef}
            className="h-[440px] overflow-auto whitespace-pre-wrap break-words font-mono text-sm"
          >
            {cortadaRef.current && output && (
              <span className="text-fd-muted-foreground">
                {"[... saida antiga cortada ...]\n"}
              </span>
            )}
            {output}
            {erro && (
              <span className="text-red-500">
                {output && !output.endsWith("\n") && "\n"}
                {erro}
              </span>
            )}
            {aviso && (
              <span className="text-amber-500">
                {(erro || (output && !output.endsWith("\n"))) && "\n"}
                {aviso}
              </span>
            )}
            {!output && !erro && !aviso && estado.tipo === "carregando" && (
              <span className="text-fd-muted-foreground">
                baixando e compilando o runtime wasm...
              </span>
            )}
            {estado.tipo === "erro" && (
              <span className="text-red-500">
                {"\n"}runtime nao carregou: {estado.mensagem}
              </span>
            )}
          </pre>
        </div>
      </div>

      <p className="mt-6 text-sm text-fd-muted-foreground">
        O codigo roda num Web Worker: se travar num laco infinito, aperta{" "}
        <strong>Parar</strong> (ou espera {TIMEOUT_PADRAO_MS / 1000}s que ele
        para sozinho). Atencao: builtins de rede/servidor/arquivo (
        <code>busca</code>, <code>escuta</code>, <code>rota</code>,{" "}
        <code>le_arquivo</code>) nao funcionam no WASM do navegador por design.
        O resto da linguagem roda normal.
      </p>
    </main>
  );
}
