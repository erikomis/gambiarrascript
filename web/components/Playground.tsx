"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import dynamic from "next/dynamic";
import { evaluate, ensureWasmRuntime } from "@/lib/wasm";
import { buttonVariants } from "fumadocs-ui/components/ui/button";

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
  { ssr: false, loading: () => <div className="h-96 animate-pulse bg-fd-muted" /> }
);

export default function Playground() {
  const [code, setCode] = useState(defaultCode);
  const [output, setOutput] = useState("");
  const [erro, setErro] = useState("");
  const [carregando, setCarregando] = useState(true);
  const [rodando, setRodando] = useState(false);
  const outRef = useRef<HTMLPreElement>(null);

  // pre-carrega o runtime WASM ao montar
  useEffect(() => {
    let cancelado = false;
    ensureWasmRuntime()
      .then(() => {
        if (!cancelado) setCarregando(false);
      })
      .catch((e) => {
        if (!cancelado) {
          setErro(String(e));
          setCarregando(false);
        }
      });
    return () => {
      cancelado = true;
    };
  }, []);

  const rodar = useCallback(async () => {
    setRodando(true);
    setErro("");
    setOutput("");
    try {
      const res = await evaluate(code);
      setOutput(res.saida ?? "");
      setErro(res.erros ?? "");
    } catch (e: any) {
      setErro(String(e?.message ?? e));
    } finally {
      setRodando(false);
      // scroll pro fim do output
      requestAnimationFrame(() => {
        if (outRef.current) outRef.current.scrollTop = outRef.current.scrollHeight;
      });
    }
  }, [code]);

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
          onClick={rodar}
          disabled={carregando || rodando}
          className={buttonVariants({ color: "primary" })}
        >
          {carregando ? "carregando wasm..." : rodando ? "rodando..." : "Rodar"}
        </button>
        <button
          onClick={() => {
            setCode("");
            setOutput("");
            setErro("");
          }}
          className={buttonVariants({ color: "outline" })}
        >
          Limpar
        </button>
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
          </div>
          <pre
            ref={outRef}
            className="h-[440px] overflow-auto whitespace-pre-wrap break-words font-mono text-sm"
          >
            {output}
            {erro && (
              <span className="text-red-500">
                {output && "\n"}
                {erro}
              </span>
            )}
            {!output && !erro && carregando && (
              <span className="text-fd-muted-foreground">compilando wasm...</span>
            )}
          </pre>
        </div>
      </div>

      <p className="mt-6 text-sm text-fd-muted-foreground">
        Atencao: builtins de rede/servidor/arquivo (<code>busca</code>,{" "}
        <code>escuta</code>, <code>rota</code>, <code>le_arquivo</code>) nao
        funcionam no WASM do navegador por design. O resto da linguagem roda
        normal.
      </p>
    </main>
  );
}