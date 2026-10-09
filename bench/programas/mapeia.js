// map + filter sobre 1 milhao de itens
const xs = [];
for (let i = 0; i < 1000000; i++) xs.push(i);
const ys = xs.map((x) => x * 2).filter((x) => x % 3 === 0);
console.log(ys.length);
console.log(ys.reduce((a, b) => a + b, 0));
