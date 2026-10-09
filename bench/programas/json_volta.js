// le um JSON de ~1 MB e faz 10 voltas de parse + serializa
const fs = require("fs");
const bruto = fs.readFileSync(process.argv[2], "utf8");
let saida = "";
let n = 0;
for (let volta = 0; volta < 10; volta++) {
  const doc = JSON.parse(bruto);
  n = doc.itens.length;
  saida = JSON.stringify(doc);
}
console.log(n);
console.log(saida.length);
console.log(saida === bruto ? "deu_bom" : "deu_ruim");
