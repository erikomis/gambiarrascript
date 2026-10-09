#!/bin/sh
# Bench entre linguagens: roda o mesmo programa em GambiarraScript (VM e
# --tree), Python 3 e Node.js, mede a mediana de N rodadas (tempo de parede,
# processo inteiro, partida inclusa) e imprime uma tabela em markdown.
#
# Uso (da raiz do repo):   sh bench/roda.sh
# Variaveis:  N=5            rodadas por medicao (mediana)
#             CARGAS="fib laco"  so essas cargas
#             SEM_TREE=1     pula a coluna do tree-walker (ele e lento)
#             GS=/caminho/gs usa esse binario em vez de compilar um
#
# Cada programa imprime um checksum; o cronometro confere que a saida de
# Python/Node bate com a do gs (senao a celula sai com "!=").
set -eu

RAIZ=$(cd "$(dirname "$0")/.." && pwd)
N=${N:-5}
CARGAS=${CARGAS:-"ola fib laco texto dicionario ordena json_volta mapeia metodo"}
TMP=$(mktemp -d "${TMPDIR:-/tmp}/gs-bench.XXXXXX")
trap 'rm -rf "$TMP"' EXIT INT TERM

cd "$RAIZ"
if [ -z "${GS:-}" ]; then
	go build -o "$TMP/gs" ./cmd/gs
	GS="$TMP/gs"
fi
go build -o "$TMP/cronometro" ./bench/cronometro

PY=$(command -v python3 || true)
NODE=$(command -v node || true)

# documento de ~1 MB pro bench de JSON (gerado pelo proprio gs)
(cd bench/programas && "$GS" roda gera_json.gs "$TMP/doc.json")

# argumentos extras por carga
extra() {
	case "$1" in
	json_volta) echo "$TMP/doc.json" ;;
	*) echo "" ;;
	esac
}

# mede: imprime a celula da tabela ("0.123" ou "0.123 !=" se a saida nao
# bater com a referencia passada em $1)
mede() {
	ref=$1
	shift
	r=$("$TMP/cronometro" -n "$N" "$@" 2>/dev/null) || {
		echo "falhou"
		return
	}
	t=${r% *}
	h=${r#* }
	if [ -n "$ref" ] && [ "$h" != "$ref" ]; then
		echo "$t !="
	else
		echo "$t"
	fi
}

maquina() {
	so=$(uname -sm)
	if command -v sysctl >/dev/null 2>&1 && sysctl -n machdep.cpu.brand_string >/dev/null 2>&1; then
		cpu=$(sysctl -n machdep.cpu.brand_string)
		nucleos=$(sysctl -n hw.ncpu)
		mem=$(($(sysctl -n hw.memsize) / 1073741824))
	else
		cpu=$(grep -m1 'model name' /proc/cpuinfo 2>/dev/null | cut -d: -f2 | sed 's/^ //')
		nucleos=$(grep -c ^processor /proc/cpuinfo 2>/dev/null || echo "?")
		mem=$(awk '/MemTotal/ {printf "%d", $2/1048576}' /proc/meminfo 2>/dev/null || echo "?")
	fi
	echo "- Maquina: $so, $cpu, $nucleos nucleos, ${mem} GB"
	echo "- gs: $("$GS" versao 2>/dev/null | head -1) ($(go version | cut -d' ' -f3))"
	[ -n "$PY" ] && echo "- Python: $("$PY" --version 2>&1)"
	[ -n "$NODE" ] && echo "- Node.js: $("$NODE" --version)"
	echo "- Mediana de $N rodadas, tempo de parede do processo inteiro (partida inclusa)"
}

maquina
echo
cab="| carga | gs (VM) |"
sep="|---|---:|"
[ -z "${SEM_TREE:-}" ] && cab="$cab gs --tree |" && sep="$sep---:|"
[ -n "$PY" ] && cab="$cab Python |" && sep="$sep---:|"
[ -n "$NODE" ] && cab="$cab Node |" && sep="$sep---:|"
[ -n "$PY" ] && cab="$cab gs/Python |" && sep="$sep---:|"
echo "$cab"
echo "$sep"

for c in $CARGAS; do
	x=$(extra "$c")
	dir="$RAIZ/bench/programas"
	ref=$(cd "$dir" && "$TMP/cronometro" -n 1 "$GS" roda "$c.gs" $x | cut -d' ' -f2)
	tgs=$(cd "$dir" && mede "$ref" "$GS" roda "$c.gs" $x)
	linha="| $c | $tgs |"
	if [ -z "${SEM_TREE:-}" ]; then
		linha="$linha $(cd "$dir" && mede "$ref" "$GS" roda --tree "$c.gs" $x) |"
	fi
	if [ -n "$PY" ]; then
		tpy=$(cd "$dir" && mede "$ref" "$PY" "$c.py" $x)
		linha="$linha $tpy |"
	fi
	[ -n "$NODE" ] && linha="$linha $(cd "$dir" && mede "$ref" "$NODE" "$c.js" $x) |"
	if [ -n "$PY" ]; then
		razao=$(awk -v a="${tgs%% *}" -v b="${tpy%% *}" 'BEGIN { if (b > 0) printf "%.1fx", a / b; else print "-" }')
		linha="$linha $razao |"
	fi
	echo "$linha"
done
