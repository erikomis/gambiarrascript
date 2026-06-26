# ROADMAP — GambiarraScript

O que falta pra linguagem ficar **realmente usável** no dia a dia. A base
(parser, avaliador, funções, closures, erros, HTTP cliente/servidor, JSON,
REPL, LSP e extensão VSCode) já está pronta — o que falta abaixo é, na maior
parte, "adicionar builtin na lista" seguindo o padrão de `interpreter/builtins.go`.

---

## Tier 1 — Essencial (sente falta na hora)

- [x] **Funções de texto**: separar, juntar, maiúsculo, minúsculo, substituir,
      fatiar, contém, começa_com / termina_com, tira_espaco (trim).
- [x] **Funções de lista**: adiciona, remove, ordena, inverte, mapeia, filtra,
      junta (lista → texto).
- [x] **Ler entrada do usuário** (stdin) — ex: `pergunta("teu nome: ")`.
      Sem isso não dá pra fazer programa de terminal interativo.

## Tier 2 — Pra projetos de verdade

- [x] **Ler/escrever arquivo** — ex: `le_arquivo(caminho)` / `escreve_arquivo(caminho, texto)`.
- [x] **Módulos / import** — hoje é tudo um arquivo só; não dá pra dividir nem
      reusar código entre arquivos. (Implementado via `importa "caminho.gs"`.)
- [x] **Math** — raiz, aleatório, arredonda, teto, chão, abs, min, max.
- [x] **Argumentos de linha de comando** (`os.Args`) acessíveis no script.
      (Builtin `argumentos()` + `gs roda arquivo.gs arg1 arg2 ...`.)

## Tier 3 — Polimento / tooling

- [x] **Hover no LSP** — mostrar a descrição de cada builtin ao passar o mouse.
- [x] **Formatador** (`gs formata arquivo.gs`).
- [x] **Mais exemplos** cobrindo cada recurso novo.
- [x] **Comentário de bloco** (hoje só tem `#` de linha).

## Tier 4 — Produção (entregue nesta leva)

- [x] **Erros robustos** — `object.Erro` carrega `Line`, `Kind`, `Stack`,
      `Cause`; `quebrou` agora amarra o próprio `Erro` (com flag `Handled`
      pra nao propagar de novo); builtins `quebra()`, `erro_msg()`,
      `erro_linha()`, `erro_tipo()`, `erro_pilha()`, `erro_causa()`,
      `envolve_erro()`. Traço de pilha impresso no `gs roda` quando o erro
      topa no nível do programa.
- [x] **Streams / stdin pesado** — `le_tudo()` (EOF completo, pra pipes),
      `le_linhas()` (lista de linhas), `escreve()` (sem \n), `escreve_erro()`
      (stderr), `anexa_arquivo()`, `env()` (var de ambiente).
- [x] **Ferramentas de debug/test** — `gs testa <dir>` roda `*_test.gs`
      somando asserts das builtins `espera()`/`afirma()`; `gs disasm
      <arquivo.gs>` imprime o bytecode; REPL agora imprime resultado de
      expressões (`=> <valor>`).
- [x] **Concorrência real** — `object.Environment` thread-safe (RWMutex);
      handler do `escuta` dispensa o lock global (roda em paralelo nas
      goroutines do net/http); builtin `paralelo(lista, fn)` aplica em
      goroutines separadas (lote 256).
- [x] **VM fase 6b** — globals (`bota`/identificador), `se_colar` com jumps e
      backpatching multi-braco, `enquanto` e `pra_cada ... de ... ate` com
      rotulos, `e`/`ou` short-circuit, `vaza`/`continua` via jumps diretos,
      `<`/`<=` reais (`OpMenor`/`OpMenorEqual`). Sem functions/builtins ainda
      (fase 6d) — exemplos puros sem função rodam igualzinhos no `gs roda --vm`.

### O que AINDA falta na VM (fases 6c-6f, nao cobertas nesta leva)

- **6c** — Listas/Dicionários literais, `OpArray`/`OpHash`, indexação.
- **6d** — Funções/locais, `OpCall`/`OpReturnValue`, frames, builtins na VM.
- **6e** — `arruma`/`quebrou` reconciliando `object.Erro` com os erros Go da VM.
- **6f** — Flip do engine default pra VM; CLI/REPL/LSP rodando na VM.

---

## ⚠️ Migrar as palavras-chave para o INGLÊS (mas continua MEME)

Decisão de direção da linguagem: **trocar as keywords (e os builtins) do
português/gambiarra para o inglês** pra ficar acessível pra fora do Brasil —
**MAS sem perder a zoeira**. Nada de `let`/`print`/`if` chatos: tem que ser
gíria/meme em inglês (estilo "no cap", "yeet", "lowkey", "dip"). A graça da
linguagem é o humor, então o inglês também tem que ser internetês.

Pontos de atenção dessa migração:

1. **Fonte da verdade** das keywords: `token/token.go` (mapa `keywords`).
2. Ao mudar, sincronizar **3 lugares**:
   - `token/token.go` — o mapa de keywords.
   - `lsp/server.go` — as listas `keywords` e `builtinsCompletion` (autocomplete).
   - `editors/vscode/syntaxes/gambiarrascript.tmLanguage.json` — as regex de cor.
   - (e `editors/vscode/snippets/gambiarrascript.json` — os snippets.)
3. Atualizar todos os `examples/*.gs` e os testes (`*_test.go`) que usam as
   keywords antigas.
4. **Sugestão:** suportar os dois (alias PT + EN) por um tempo, pra não quebrar
   os scripts existentes — `LookupIdent` pode mapear ambos pro mesmo token.

### Tabela de tradução sugerida (keywords)

| GambiarraScript (atual) | Inglês sugerido |
|-------------------------|-----------------|
| `bota`                  | `let`           |
| `mostra`                | `print`         |
| `se_colar`              | `if`            |
| `se_nao_colar`          | `else`          |
| `enquanto`              | `while`         |
| `pra_cada`              | `for`           |
| `de`                    | `from`          |
| `ate`                   | `to`            |
| `em`                    | `in`            |
| `gambiarra`             | `func`          |
| `funciona`              | `return`        |
| `arruma`                | `try`           |
| `quebrou`               | `catch`         |
| `vaza`                  | `break`         |
| `continua`              | `continue`      |
| `deu_bom`               | `true`          |
| `deu_ruim`              | `false`         |
| `nada`                  | `nil`           |
| `acabou_finalmente`     | `end`           |
| `e`                     | `and`           |
| `ou`                    | `or`            |
| `nao`                   | `not`           |

### Tabela de tradução sugerida (builtins)

| Atual      | Inglês sugerido |
|------------|-----------------|
| `tamanho`  | `length`        |
| `chaves`   | `keys`          |
| `tem`      | `has`           |
| `texto`    | `string`        |
| `numero`   | `number`        |
| `busca`    | `fetch`         |
| `rota`     | `route`         |
| `escuta`   | `listen`        |
| `de_json`  | `parse_json`    |
| `pra_json` | `to_json`       |

> Obs: trocar pro inglês descaracteriza o tema "gambiarra/zoeira BR". Avaliar se
> a ideia é **substituir** de vez ou **adicionar inglês como alias** mantendo o
> português como identidade da linguagem.
