// Teste ponta a ponta do playground em Chromium, Firefox e WebKit (Safari).
// Uso: BASE=http://localhost:PORTA/gambiarrascript node e2e/navegadores.cjs
// (SO=firefox,webkit escolhe os motores). Sai com 1 se algum passo falhar.
const { firefox, webkit, chromium } = require("playwright");
const zlib = require("zlib");
const BASE = process.env.BASE;
const link = (c) => `${BASE}/playground/#c=` + zlib.deflateRawSync(Buffer.from(c)).toString("base64url");
const saida = (p) => p.locator("pre").last().innerText();
async function prontoERoda(p) {
  await p.getByText("baixando e compilando o runtime wasm").first().waitFor({ state: "hidden", timeout: 90000 }).catch(() => {});
  await p.getByRole("button", { name: "Rodar" }).click({ timeout: 90000 });
}
async function espera(p, trecho, ms) {
  const fim = Date.now() + ms;
  while (Date.now() < fim) { if ((await saida(p)).includes(trecho)) return true; await p.waitForTimeout(200); }
  throw new Error(`nao apareceu "${trecho}" em ${ms}ms; saida: ${(await saida(p)).slice(0, 200)}`);
}
async function testa(tipo, opts) {
  const nav = await tipo.launch(opts);
  const p = await nav.newPage();
  const erros = [];
  p.on("pageerror", (e) => erros.push(e.message));
  const r = {};
  const passo = async (nome, fn) => { try { await fn(); r[nome] = "ok"; } catch (e) { r[nome] = "FALHOU: " + e.message.split("\n")[0]; } };
  await passo("exemplo padrao", async () => { await p.goto(`${BASE}/playground/`); await prontoERoda(p); await espera(p, "dobro de 3 = 6", 30000); });
  await passo("busca avisa na hora", async () => {
    await p.goto(link('bota r = busca("https://httpbin.org/get")\n')); await p.reload(); await prontoERoda(p);
    await espera(p, "nao roda no navegador", 3000);
  });
  await passo("parar laco infinito", async () => {
    await p.goto(link("enquanto deu_bom\n    bota x = 1\nacabou_finalmente\n")); await p.reload(); await prontoERoda(p);
    await p.waitForTimeout(1000); await p.getByRole("button", { name: "Parar" }).click(); await espera(p, "interrompida", 5000);
    await p.getByRole("button", { name: "Salve, tropa" }).click(); await p.getByRole("button", { name: "Rodar" }).click(); await espera(p, "Salve, tropa!", 30000);
  });
  await passo("entrada (pergunta)", async () => {
    await p.getByRole("button", { name: "Quiz (pergunta)" }).click(); await p.getByRole("button", { name: "Rodar" }).click(); await espera(p, "teu nome:", 15000);
  });
  await passo("botao da doc", async () => {
    await p.goto(`${BASE}/docs/strings/`); await p.getByRole("link", { name: /Rodar no playground/ }).first().click();
    await p.waitForURL(/playground/); await prontoERoda(p);
    const fim = Date.now() + 30000; while (Date.now() < fim && !(await saida(p)).trim()) await p.waitForTimeout(200);
    if (!(await saida(p)).trim()) throw new Error("saida vazia");
  });
  r["erros de pagina"] = erros.length ? erros.join(" | ").slice(0, 200) : "nenhum";
  await nav.close();
  return r;
}
(async () => {
  const motores = (process.env.SO || "chromium,firefox,webkit").split(",").map((n) => [n, { chromium, firefox, webkit }[n], {}]);
  let falhou = false;
  for (const [nome, tipo, opts] of motores) {
    const r = await testa(tipo, opts);
    console.log(nome, JSON.stringify(r, null, 1));
    if (Object.entries(r).some(([k, v]) => k !== "erros de pagina" && v !== "ok")) falhou = true;
  }
  process.exit(falhou ? 1 : 0);
})();
