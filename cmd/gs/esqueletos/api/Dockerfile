# Imagem de producao: o gs oficial (distroless, usuario nonroot, /app) + o
# projeto. Nada de compilar: o script roda na VM do gs.
#
#   docker build -t __NOME__ .
#   docker run --rm -p 8080:8080 -e JWT_SEGREDO="$(openssl rand -base64 32)" \
#       -v __NOME__-dados:/app/dados __NOME__
#
# O volume em /app/dados guarda o banco sqlite entre um container e outro.
FROM ghcr.io/erikomis/gambiarrascript:__VERSAO_GS__

# --chown: o usuario nonroot precisa gravar o banco em /app/dados
COPY --chown=nonroot:nonroot . /app

ENV PORTA=8080 \
    GS_BANCO=sqlite:dados/app.db \
    GS_LOG_FORMATO=json

EXPOSE 8080
CMD ["roda", "principal.gs"]
