# Imagem oficial do GambiarraScript (ghcr.io/erikomis/gambiarrascript).
#
# Quem monta e o workflow release.yml: ele NAO compila nada aqui dentro, so
# copia o binario estatico (CGO_ENABLED=0) que o scripts/release ja gerou pra
# cada plataforma. Contexto esperado (o workflow prepara em dist/docker/):
#
#   linux_amd64/gs
#   linux_arm64/gs
#   app/            pasta vazia (vira o /app do usuario nonroot)
#
# Base distroless "static": sem shell e sem gerenciador de pacote, mas com
# certificados (busca/https), tzdata (agenda, converte_tz) e o usuario
# nonroot (uid 65532). Sem RUN nenhum, entao o build multi-arch nao precisa
# de QEMU. Precisa de shell pra depurar? Copie o gs pra sua propria imagem:
#   COPY --from=ghcr.io/erikomis/gambiarrascript /usr/local/bin/gs /usr/local/bin/gs
#
# Uso:
#   docker run --rm ghcr.io/erikomis/gambiarrascript versao
#   docker run --rm -v "$PWD:/app" ghcr.io/erikomis/gambiarrascript roda app.gs
#
# (o Dockerfile da raiz do repo e outra coisa: build de desenvolvimento.)
FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETARCH
ARG VERSAO=dev

LABEL org.opencontainers.image.title="GambiarraScript" \
      org.opencontainers.image.description="A linguagem de programacao do jeitinho brasileiro" \
      org.opencontainers.image.source="https://github.com/erikomis/gambiarrascript" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSAO}"

COPY linux_${TARGETARCH}/gs /usr/local/bin/gs
# /app do usuario nonroot: da pra gravar banco sqlite, log, upload...
COPY --chown=65532:65532 app/ /app/

WORKDIR /app
USER nonroot
ENTRYPOINT ["gs"]
CMD ["ajuda"]
