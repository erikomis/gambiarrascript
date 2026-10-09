# __NOME__

API REST de tarefas com cadastro, login com JWT e banco SQLite, feita com
[GambiarraScript](https://github.com/erikomis/gambiarrascript).

## Rodando

```bash
cp .env.exemplo .env          # opcional: porta, banco, segredo do JWT
gs roda principal.gs          # sobe na porta 8080 (ou a do PORTA)
gs roda principal.gs --porta 3000
gs roda principal.gs --ajuda  # todas as flags
```

Na subida ele conecta no banco (`GS_BANCO`, padrao `sqlite:dados/app.db`) e
aplica as migracoes pendentes de `migracoes/`. Cada pedido vira uma linha de
log no stderr (`GS_LOG_FORMATO=json` pra sair em JSON).

## Usando

```bash
# cadastro (201) — senha de 8 a 72 caracteres
curl -X POST localhost:8080/usuarios -H "Content-Type: application/json" \
     -d '{"nome": "Ju", "email": "ju@exemplo.com", "senha": "segredo123"}'

# login: devolve {"token": "..."}
TOKEN=$(curl -s -X POST localhost:8080/login -H "Content-Type: application/json" \
     -d '{"email": "ju@exemplo.com", "senha": "segredo123"}' | sed 's/.*"token":"\([^"]*\)".*/\1/')

curl localhost:8080/eu -H "Authorization: Bearer $TOKEN"

# tarefas (cada usuario so ve as suas)
curl -X POST localhost:8080/tarefas -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" -d '{"titulo": "lavar a louca"}'
curl localhost:8080/tarefas -H "Authorization: Bearer $TOKEN"
curl -X PUT localhost:8080/tarefas/1 -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" -d '{"feita": true}'
curl -X DELETE localhost:8080/tarefas/1 -H "Authorization: Bearer $TOKEN"
```

| Rota | Token? | O que faz |
| --- | --- | --- |
| `GET /` e `GET /saude` | nao | info da API / checagem de vida |
| `POST /usuarios` | nao | cadastro (`nome`, `email`, `senha`) |
| `POST /login` | nao | devolve o JWT (vale 24h) |
| `GET /eu` | sim | o usuario do token |
| `GET /tarefas`, `POST /tarefas` | sim | lista / cria (`titulo`, `feita`) |
| `GET`, `PUT`, `DELETE /tarefas/:id` | sim | le / muda / apaga |

Erros vem em JSON: `400 {"erros": [...]}` quando o corpo nao passa no
`valida()`, `401` sem token (ou token vencido), `404` quando a tarefa nao
existe (ou e de outro usuario) e `409` no cadastro com email repetido.

## Estrutura

```
principal.gs          config, banco, middleware (log + JWT), cors e escuta
rotas/usuarios.gs     cadastro, login e /eu
rotas/tarefas.gs      CRUD de tarefas
lib/http.gs           ajudantes sem efeito colateral (rotas publicas, token, :id)
migracoes/            SQL versionado (NNN_nome.sobe.sql / .desce.sql)
*_test.gs             testes (gs testa)
```

Rota nova sem login? Poe em `ROTAS_PUBLICAS` (`lib/http.gs`). Modulo novo de
rotas? Crie `rotas/x.gs` com um `registra(db)` e chame no `principal.gs`.

## Testando e conferindo

```bash
gs testa            # roda os *_test.gs
gs check principal.gs rotas/*.gs lib/*.gs
gs formata -w .     # formata todos os .gs
```

## Migracoes

```bash
gs migra novo poe_prazo_nas_tarefas   # cria migracoes/003_poe_prazo_nas_tarefas.sobe.sql e .desce.sql
gs migra status --banco sqlite:dados/app.db
gs migra desce --banco sqlite:dados/app.db   # desfaz a ultima
```

O servidor aplica as pendentes sozinho na subida (`migra(db)`); o `gs migra`
e pra criar, ver o status e desfazer. Migracao aplicada nao se edita: crie
uma nova.

## Deploy

Com Docker (imagem oficial do `gs`, sem nada pra compilar):

```bash
docker build -t __NOME__ .
docker run --rm -p 8080:8080 -e JWT_SEGREDO="$(openssl rand -base64 32)" \
    -v __NOME__-dados:/app/dados __NOME__
```

Sem Docker, um binario so com tudo dentro:

```bash
gs build principal.gs -o __NOME__
gs build principal.gs --alvo linux/amd64 -o __NOME__   # pra um servidor linux
```

O binario leva os `.gs`, mas nao os `.sql`: copie a pasta `migracoes/` pro
lado dele (o `migra(db)` le de la, relativo a pasta onde ele roda). Em
producao:

- defina `JWT_SEGREDO` (sem ele cada subida sorteia um e os tokens morrem);
- deixe o TLS com um proxy reverso (Caddy, nginx) ou use
  `escuta(porta, {"tls": {"cert": "...", "chave": "..."}})`;
- troque o `cors()` aberto por `cors({"origens": ["https://teu-site.com"]})`.
