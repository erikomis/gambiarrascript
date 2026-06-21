# 🇧🇷 GambiarraScript

> A linguagem de programação do jeitinho brasileiro. Escrita em Go.

GambiarraScript é uma linguagem onde você não fecha bloco com `}` nem com `end` — você fecha com **`acabou_finalmente`**, porque programar no Brasil é isso: deu trabalho, mas graças a Deus acabou.

## Salve, tropa

```
mostra "Salve, tropa!"

bota nome = "Erik"
bota idade = 25

se_colar idade >= 18
    mostra nome + " pode entrar"
se_nao_colar
    mostra "volta daqui a pouco"
acabou_finalmente
```

## Vocabulário

| GambiarraScript     | O que faz           |
|---------------------|---------------------|
| `bota`              | declara variável    |
| `mostra`            | imprime na tela     |
| `se_colar` / `se_nao_colar` | if / else / else-if |
| `enquanto`          | while               |
| `pra_cada i de 1 ate 10` | for numérico   |
| `pra_cada x em lista`    | for-each       |
| `gambiarra`         | declara função      |
| `funciona`          | return              |
| `arruma` / `quebrou`| try / catch         |
| `vaza` / `continua` | break / continue    |
| `deu_bom` / `deu_ruim` | true / false     |
| `nada`              | null                |
| `acabou_finalmente` | fecha o bloco       |

## Quando deu ruim, a gente arruma

```
arruma
    bota resultado = 10 / 0
quebrou erro
    mostra "deu ruim, parca: " + erro
acabou_finalmente
```

## Como rodar (só precisa de Docker)

Sem instalar Go. O helper `scripts/dgo` roda tudo num container.

```bash
# rodar um arquivo
./scripts/dgo run ./cmd/gs roda examples/fizzbuzz.gs

# abrir o REPL interativo
docker run --rm -it -v "$PWD":/app -w /app golang:1.23 go run ./cmd/gs repl
```

## Rodando os testes

```bash
./scripts/dgo test ./...
```

Feito na gambiarra, com carinho. 🛠️
