// 1 milhao de chamadas de metodo num objeto (class)
class Contador {
  constructor() {
    this.n = 0;
  }
  soma(k) {
    this.n += k;
  }
}
const c = new Contador();
for (let i = 0; i < 1000000; i++) c.soma(i % 3);
console.log(c.n);
