# __NOME__

Site com paginas renderizadas no servidor, feito com
[GambiarraScript](https://github.com/erikomis/gambiarrascript): modelos HTML
com layout, parciais e laco, mais CSS servido como arquivo estatico.

## Rodando

```bash
gs roda principal.gs              # http://localhost:8080
gs roda principal.gs --porta 3000
```

Rode **de dentro da pasta do projeto**: os caminhos `modelos/` e
`estatico/` sao relativos a ela. Editou um `.html`? A proxima requisicao ja
ve (o modelo recompila sozinho quando o arquivo muda); mudou um `.gs`,
reinicia.

## Estrutura

```text
principal.gs              rotas: /, /artigos/:slug, /busca?q=, /sobre
conteudo.gs               os artigos (lista fixa; troque por um banco quando crescer)
modelos/base.html         layout: blocos "titulo" e "conteudo"
modelos/parciais/         cabecalho, rodape e o cartao de artigo
modelos/*.html            as paginas (usa "base.html" + blocos)
estatico/                 CSS e afins, servidos em /estatico/ (serve_pasta)
conteudo_test.gs          testes (gs testa)
```

Pagina nova: crie `modelos/x.html` comecando com `{{ usa "base.html" }}` e
uma rota que chama `pagina("x.html", dados)`. Todo valor `{{ ... }}` sai
escapado (protecao contra XSS); HTML cru so com `{{{ ... }}}`, e nunca com
texto que veio de fora.

## Testando e conferindo

```bash
gs testa
gs check principal.gs conteudo.gs
gs formata -w .
```

## Deploy

```bash
docker build -t __NOME__ .
docker run --rm -p 8080:8080 __NOME__
```

Ou um binario: `gs build principal.gs -o __NOME__` (leve `modelos/` e
`estatico/` pro lado dele — o binario so embute os `.gs`).
