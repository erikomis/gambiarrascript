// 200 mil chaves texto: insere tudo, depois le tudo de volta
// (Map e o dicionario de verdade do JS; objeto literal com 200k chaves
// vira modo dicionario do V8 e e mais lento)
const d = new Map();
for (let i = 0; i < 200000; i++) d.set("k" + String(i), i);
let s = 0;
for (let i = 0; i < 200000; i++) s += d.get("k" + String(i));
console.log(d.size);
console.log(s);
