// monta um texto de 100 mil caracteres concatenando um por vez
let s = "";
for (let i = 0; i < 100000; i++) s += String(i % 10);
console.log(s.length);
console.log(s.slice(-10));
