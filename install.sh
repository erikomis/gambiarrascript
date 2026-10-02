#!/bin/sh
# Instalador do GambiarraScript (gs) pra macOS e Linux.
#
#   curl -fsSL https://raw.githubusercontent.com/erikomis/gambiarrascript/main/install.sh | sh
#
# Variaveis opcionais:
#   GS_VERSAO=0.2.0   instala essa versao em vez da ultima
#   GS_DIR=~/bin      instala nesse diretorio (padrao: /usr/local/bin se der
#                     pra escrever, senao ~/.local/bin). Nunca usa sudo.
#   GS_URL_BASE=...   baixa os arquivos dessa URL base em vez do GitHub
#                     (pra testes/espelhos; exige GS_VERSAO)
#
# Tudo dentro de main() pra um download cortado no meio nao rodar pela metade.

set -eu

REPO="erikomis/gambiarrascript"

diz() { printf '%s\n' "gs: $*"; }
morre() { printf '%s\n' "gs: deu ruim: $*" >&2; exit 1; }
tem() { command -v "$1" >/dev/null 2>&1; }

baixa() { # baixa <url> <arquivo>
  if tem curl; then
    curl -fsSL --retry 3 -o "$2" "$1"
  elif tem wget; then
    wget -q -O "$2" "$1"
  else
    morre "preciso de curl ou wget pra baixar as coisas"
  fi
}

sha256_de() { # sha256_de <arquivo>
  if tem sha256sum; then
    sha256sum "$1" | cut -d' ' -f1
  elif tem shasum; then
    shasum -a 256 "$1" | cut -d' ' -f1
  else
    morre "preciso de sha256sum ou shasum pra conferir o download"
  fi
}

detecta_os() {
  case "$(uname -s)" in
    Darwin) echo darwin ;;
    Linux) echo linux ;;
    MINGW* | MSYS* | CYGWIN*)
      morre "no Windows baixa o .zip direto em https://github.com/$REPO/releases" ;;
    *) morre "sistema '$(uname -s)' nao suportado (so macOS e Linux, por enquanto)" ;;
  esac
}

detecta_arch() {
  arch=$(uname -m)
  case "$arch" in
    x86_64 | amd64) arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *) morre "arquitetura '$arch' nao suportada (so amd64 e arm64)" ;;
  esac
  # shell rodando no Rosetta num Mac M*: o binario nativo arm64 e melhor
  if [ "$arch" = amd64 ] && [ "$(uname -s)" = Darwin ] &&
    [ "$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)" = 1 ]; then
    arch=arm64
  fi
  echo "$arch"
}

ultima_versao() {
  json="$tmp/latest.json"
  baixa "https://api.github.com/repos/$REPO/releases/latest" "$json" ||
    morre "nao consegui descobrir a ultima versao no GitHub (nenhuma release publicada ainda? sem internet? limite da API?). Tenta GS_VERSAO=x.y.z"
  tag=$(sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' "$json" | head -n 1)
  [ -n "$tag" ] || morre "o GitHub respondeu mas nao achei nenhuma release publicada"
  echo "${tag#v}"
}

escolhe_dir() {
  if [ -n "${GS_DIR:-}" ]; then
    echo "$GS_DIR"
  elif [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
    echo /usr/local/bin
  else
    echo "$HOME/.local/bin"
  fi
}

main() {
  os=$(detecta_os)
  arch=$(detecta_arch)

  tmp=$(mktemp -d 2>/dev/null || mktemp -d -t gs-install)
  trap 'rm -rf "$tmp"' EXIT
  trap 'exit 130' INT TERM

  if [ -n "${GS_VERSAO:-}" ]; then
    versao=${GS_VERSAO#v}
  elif [ -n "${GS_URL_BASE:-}" ]; then
    morre "GS_URL_BASE precisa de GS_VERSAO junto"
  else
    diz "perguntando pro GitHub qual e a ultima versao..."
    versao=$(ultima_versao)
  fi

  base=${GS_URL_BASE:-"https://github.com/$REPO/releases/download/v$versao"}
  arquivo="gs_${versao}_${os}_${arch}.tar.gz"

  diz "baixando GambiarraScript $versao pra $os/$arch (segura o cafe)..."
  baixa "$base/$arquivo" "$tmp/$arquivo" ||
    morre "nao achei $arquivo em $base (essa versao existe mesmo?)"
  baixa "$base/checksums.txt" "$tmp/checksums.txt" ||
    morre "nao consegui baixar o checksums.txt"

  esperado=$(awk -v f="$arquivo" '$2 == f { print $1 }' "$tmp/checksums.txt")
  [ -n "$esperado" ] || morre "$arquivo nao aparece no checksums.txt"
  obtido=$(sha256_de "$tmp/$arquivo")
  if [ "$esperado" != "$obtido" ]; then
    morre "o sha256 nao bate! esperado $esperado, veio $obtido. Gambiarra tem limite: nao vou instalar isso"
  fi
  diz "sha256 conferido, ta limpo"

  tar -xzf "$tmp/$arquivo" -C "$tmp" gs || morre "o pacote veio quebrado"

  dir=$(escolhe_dir)
  mkdir -p "$dir" 2>/dev/null || true
  if [ ! -d "$dir" ] || [ ! -w "$dir" ]; then
    morre "sem permissao pra escrever em $dir. Escolhe outro com GS_DIR=~/.local/bin (sudo eu nao uso escondido)"
  fi

  # copia pra um temporario no destino e troca com mv: arquivo novo (inode
  # novo), entao o macOS nao mata o binario por cache de assinatura velha
  cp "$tmp/gs" "$dir/.gs.novo.$$"
  chmod 755 "$dir/.gs.novo.$$"
  mv -f "$dir/.gs.novo.$$" "$dir/gs"

  if [ "$os" = darwin ] && tem xattr; then
    xattr -d com.apple.quarantine "$dir/gs" 2>/dev/null || true
  fi

  diz "instalado em $dir/gs"

  case ":$PATH:" in
    *":$dir:"*) ;;
    *)
      diz "atencao: $dir nao ta no seu PATH. Bota isso no seu ~/.zshrc ou ~/.bashrc:"
      # shellcheck disable=SC2016 # o $PATH literal e de proposito
      printf '\n    export PATH="%s:$PATH"\n\n' "$dir"
      ;;
  esac

  "$dir/gs" --version || morre "instalou mas o gs nao quis rodar. Abre uma issue em https://github.com/$REPO/issues"
  diz "pronto! roda 'gs repl' e bora gambiarrar"
}

main "$@"
