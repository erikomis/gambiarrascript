// API JSON minima pro teste de carga (bench/carga), com o http embutido do
// Node (sem framework). Mesmo contrato do api.gs e do api.py.
// Uso: node bench/http/api.js 8080
const http = require("http");

const itens = new Map();
for (let i = 1; i <= 1000; i++) itens.set(String(i), { id: i, nome: `item ${i}`, preco: i * 2 });
let proximo = 1001;

function json(res, status, v) {
  const corpo = JSON.stringify(v);
  res.writeHead(status, { "Content-Type": "application/json", "Content-Length": Buffer.byteLength(corpo) });
  res.end(corpo);
}

http
  .createServer((req, res) => {
    if (req.method === "GET" && req.url === "/ping") return json(res, 200, { ok: true });
    if (req.method === "GET" && req.url.startsWith("/itens/")) {
      const it = itens.get(req.url.slice(7));
      return it ? json(res, 200, it) : json(res, 404, { erro: "nao achei" });
    }
    if (req.method === "POST" && req.url === "/itens") {
      let corpo = "";
      req.on("data", (c) => (corpo += c));
      req.on("end", () => {
        let dados;
        try {
          dados = JSON.parse(corpo);
        } catch {
          return json(res, 400, { erro: "json quebrado" });
        }
        const id = proximo++;
        const it = { id, nome: dados.nome, preco: dados.preco };
        itens.set(String(id), it);
        json(res, 201, it);
      });
      return;
    }
    json(res, 404, { erro: "rota nao encontrada" });
  })
  .listen(Number(process.argv[2] || 8080), "127.0.0.1");
