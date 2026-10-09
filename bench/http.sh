#!/bin/sh
# Teste de carga HTTP: a mesma API JSON (bench/http/api.*) em GambiarraScript
# (VM e --tree), Node.js (http embutido) e Python (aiohttp, se instalado),
# marteladas pelo bench/carga (C conexoes keep-alive por D). Imprime uma
# tabela markdown com req/s e latencia p50/p99.
#
# Uso (da raiz do repo):  sh bench/http.sh
# Variaveis: C=50  D=10s  PORTA=18080  GS=/caminho/gs  SEM_TREE=1
#
# O gerador de carga roda na MESMA maquina e disputa CPU com o servidor: o
# numero absoluto vale pra comparar entre si, nao como capacidade de producao.
set -eu

RAIZ=$(cd "$(dirname "$0")/.." && pwd)
C=${C:-50}
D=${D:-10s}
PORTA=${PORTA:-18080}
TMP=$(mktemp -d "${TMPDIR:-/tmp}/gs-http.XXXXXX")
PID=""
limpa() {
	if [ -n "$PID" ]; then
		kill "$PID" 2>/dev/null || true
		wait "$PID" 2>/dev/null || true
	fi
	rm -rf "$TMP"
}
trap limpa EXIT INT TERM

cd "$RAIZ"
if [ -z "${GS:-}" ]; then
	go build -o "$TMP/gs" ./cmd/gs
	GS="$TMP/gs"
fi
go build -o "$TMP/carga" ./bench/carga

# mede <nome> <comando...>: sobe o servidor, roda os 3 modos, derruba
mede() {
	nome=$1
	shift
	"$@" "$PORTA" >/dev/null 2>&1 &
	PID=$!
	# aquecimento (JIT do Node, caches): 1s de ping que nao entra na conta
	"$TMP/carga" -url "http://127.0.0.1:$PORTA" -modo ping -c "$C" -d 1s >/dev/null || true
	for modo in ping item cria; do
		r=$("$TMP/carga" -url "http://127.0.0.1:$PORTA" -modo "$modo" -c "$C" -d "$D" || echo "falhou - - -")
		set -- $r
		echo "| $nome | $modo | $1 | $2 | $3 | $4 |"
	done
	kill "$PID" 2>/dev/null || true
	wait "$PID" 2>/dev/null || true
	PID=""
	PORTA=$((PORTA + 1))
}

echo "- $C conexoes, $D por modo, gerador de carga na mesma maquina"
echo
echo "| servidor | modo | req/s | p50 (ms) | p99 (ms) | falhas |"
echo "|---|---|---:|---:|---:|---:|"
mede "gs (VM)" "$GS" roda bench/http/api.gs
[ -z "${SEM_TREE:-}" ] && mede "gs --tree" "$GS" roda --tree bench/http/api.gs
if command -v node >/dev/null 2>&1; then
	mede "Node (http)" node bench/http/api.js
fi
if command -v python3 >/dev/null 2>&1 && python3 -c "import aiohttp" 2>/dev/null; then
	mede "Python (aiohttp)" python3 bench/http/api.py
fi
