# Imagem de producao: o gs oficial (distroless, usuario nonroot, /app) + o site.
#
#   docker build -t __NOME__ .
#   docker run --rm -p 8080:8080 __NOME__
FROM ghcr.io/erikomis/gambiarrascript:__VERSAO_GS__
COPY --chown=nonroot:nonroot . /app
ENV PORTA=8080 \
    GS_LOG_FORMATO=json
EXPOSE 8080
CMD ["roda", "principal.gs"]
