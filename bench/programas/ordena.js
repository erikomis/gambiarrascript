// 300 mil inteiros pseudo-aleatorios (MINSTD, semente 42 — o mesmo gerador
// nas 3 linguagens, pra ordenar exatamente a mesma lista) e um sort
// (x * 48271 cabe em 2^53, entao a conta e exata em double)
let x = 42;
const xs = [];
for (let i = 0; i < 300000; i++) {
  x = (x * 48271) % 2147483647;
  xs.push(x % 1000000);
}
xs.sort((a, b) => a - b);
console.log(xs[0]);
console.log(xs[150000]);
console.log(xs[299999]);
